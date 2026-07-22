# Docs package: nav tree → directory bundle → Go viewer.
# Generated options use pkgs.nixosOptionsDoc; option records never enter JSON.
{ lib
, writeText
, buildGoModule
, runCommand
, nixosOptionsDoc
, figlet
, ...
}:

# Component config from module: shared palette fields + docs.pages + docs.nixosOptions
config:

let
  d = import ./defaults.nix;
  plib = import ./lib.nix { inherit lib; };

  pal = plib.resolvePalette (config.theme or d.theme) (config.palette or d.palette);
  colorProfile = config.colorProfile or d.colorProfile;
  project = config.project or d.project;
  pages = config.pages or [ ];
  rootReadmePath = config.rootReadme or null;
  # Smaller wordmark font for the root-README hero. No user customization —
  # a single fixed small font keeps the bundle reproducible and the width
  # predictable. Rendered at build time by figlet into hero.txt and baked
  # into the config bundle as a store path (no IFD: the Go viewer reads it
  # at runtime, Nix never reads the contents).
  heroFont = ./fonts/small-slant.flf;
  heroText = project;

  # Full nixosOptionsDoc argument set. Pass through for split=shallow;
  # allLeaves builds one leaf page per terminal option from the same source.
  nixosOptionsArgs = removeAttrs (config.nixosOptions or { options = { }; }) [
    # Module system internals if a freeform submodule ever leaks them.
    "_module"
  ];

  opts = nixosOptionsArgs.options or { };

  # Same transform nixosOptionsDoc applies (default lib.id).
  transformOptions = nixosOptionsArgs.transformOptions or lib.id;

  # Flattened option docs — same pipeline as nixosOptionsDoc before Markdown:
  # optionAttrSetToDocList → transformOptions → drop invisible/internal/_module.
  #
  # Each entry keeps BOTH locs:
  #   lookupLoc  = raw loc (always valid for optionAt / nestPath)
  #   displayLoc = transformed loc (nav titles)
  # A caller transform may rewrite name/loc; lookup must never follow that rename.
  docList =
    let
      rawList = lib.optionAttrSetToDocList opts;
      paired = map
        (raw: {
          inherit raw;
          transformed = transformOptions raw;
        })
        rawList;
      keep =
        { raw
        , transformed
        ,
        }:
        (transformed.visible or true)
        && !(transformed.internal or false)
        && !(lib.any (s: s == "_module") (raw.loc or [ ]))
        && !(lib.any (s: s == "_module") (transformed.loc or [ ]));
      kept = builtins.filter keep paired;
    in
    map
      (
        { raw
        , transformed
        ,
        }:
        {
          lookupLoc = raw.loc;
          displayLoc = transformed.loc or raw.loc;
          displayName = transformed.name or (lib.concatStringsSep "." (transformed.loc or raw.loc));
        }
      )
      kept;

  nodeShapeOK =
    node:
    let
      hasText = (node.text or null) != null;
      hasChildren = (node.children or [ ]) != [ ];
      hasGenerate = (node.generate or null) != null;
      hasTitle = (node.title or null) != null;
    in
    if hasGenerate then
    # Generate node: no text/children.
      !hasText && !hasChildren
    else if hasChildren then
    # Group: title required. Optional `text` is provenance only (mdSplit
    # original path for rootReadme/FIGlet) — not a body.
      hasTitle
    else
    # Leaf: text required.
      hasText;

  # nestPath [ "prelude" "motd" ] value → { prelude = { motd = value; }; }
  nestPath =
    path: value:
    lib.foldr (seg: acc: { ${seg} = acc; }) value path;

  isPlaceholder =
    seg: seg == "*" || (lib.hasPrefix "<" seg && lib.hasSuffix ">" seg);

  # Recover the raw option record at `path` by walking getSubOptions
  # segment-by-segment. lib.attrByPath cannot do this: opts.prelude is an
  # option record, not an attrset of children.
  #
  # optionAttrSetToDocList inserts placeholder segments ("<name>", "*") into
  # loc for attrsOf/listOf submodules. Those are NOT keys in getSubOptions
  # output — skip them and keep walking the current attrs.
  optionAt =
    path:
    let
      go =
        prefix: attrs: segs:
        if segs == [ ] then
          null
        else
          let
            seg = builtins.head segs;
            rest = builtins.tail segs;
          in
          if isPlaceholder seg then
            go (prefix ++ [ seg ]) attrs rest
          else
            let
              cur = attrs.${seg} or null;
              nextPrefix = prefix ++ [ seg ];
            in
            if cur == null then
              null
            else if rest == [ ] then
              cur
            else if lib.isOption cur && ((cur.type.getSubOptions or null) != null) then
              go nextPrefix (removeAttrs (cur.type.getSubOptions nextPrefix) [ "_module" ]) rest
            else if (!(lib.isOption cur)) && builtins.isAttrs cur then
              go nextPrefix (removeAttrs cur [ "_module" ]) rest
            else
              null;
    in
    go [ ] opts path;

  # Single-option leaf. lookupLoc is always the *raw* path.
  optionLeaf =
    title: lookupLoc:
    let
      val = optionAt lookupLoc;
      doc = nixosOptionsDoc (
        nixosOptionsArgs
        // {
          options =
            assert lib.assertMsg (val != null) ''
              docs: optionAt ${lib.concatStringsSep "." lookupLoc} returned null —
              cannot build nixosOptionsDoc leaf (getSubOptions walk failed).
            '';
            nestPath lookupLoc val;
        }
      );
    in
    {
      kind = "leaf";
      inherit title;
      markdownPath = doc.optionsCommonMark;
      gapBefore = false;
      rootReadme = false;
    };

  asGroup =
    title: children:
    {
      kind = "group";
      inherit title children;
      gapBefore = true;
      rootReadme = false;
    };

  # Nested nav from displayLoc; lookupLoc drives optionAt on leaves.
  treeFromDocList =
    let
      insertFull =
        trie: entry: loc:
        if loc == [ ] then
          trie
        else
          let
            seg = builtins.head loc;
            rest = builtins.tail loc;
            child = trie.${seg} or {
              children = { };
              isLeaf = false;
              entry = null;
            };
            updated =
              if rest == [ ] then
                child
                // {
                  isLeaf = true;
                  entry = entry;
                }
              else
                child
                // {
                  children = insertFull child.children entry rest;
                };
          in
          trie // { ${seg} = updated; };

      trie = lib.foldl' (t: e: insertFull t e e.displayLoc) { } docList;

      emit =
        name: node:
        let
          childNames = lib.attrNames node.children;
          childNodes = map (c: emit c node.children.${c}) (lib.sort (a: b: a < b) childNames);
        in
        if node.isLeaf && childNames == [ ] then
          optionLeaf node.entry.displayName node.entry.lookupLoc
        else
          {
            kind = "group";
            title = name;
            children = childNodes;
            gapBefore = false;
            rootReadme = false;
          };

      roots = lib.attrNames trie;
    in
    map (r: emit r trie.${r}) (lib.sort (a: b: a < b) roots);

  # Generate expansion (see prelude.docs.pages.*.split).
  # Default allLeaves; shallow = one full nixosOptionsDoc page.
  expandGenerate =
    node:
    let
      title = if (node.title or null) != null then node.title else "Options";
      split = node.split or "allLeaves";
    in
    assert lib.assertMsg (opts != { }) ''
      docs: pages node has generate = "nixosOptions" but prelude.docs.nixosOptions.options is empty.
      Set prelude.docs.nixosOptions = { inherit (eval) options; } (nixosOptionsDoc argument shape).
    '';
    if split == "shallow" then
      {
        kind = "leaf";
        inherit title;
        markdownPath = (nixosOptionsDoc nixosOptionsArgs).optionsCommonMark;
        gapBefore = true;
        rootReadme = false;
      }
    else
    # split = "allLeaves" (default): nested sidebar of every terminal option.
      let
        tree = treeFromDocList;
      in
      assert lib.assertMsg (tree != [ ]) ''
        docs: split = "allLeaves" produced no option leaves (docList empty after visibility filter).
      '';
      asGroup title tree;

  # Mark leaves whose text path is exactly prelude.docs.rootReadme (path equality
  # before collect renames sources to pages/NNN.md). No basename guessing.
  leafIsRootReadme =
    path:
    rootReadmePath != null && path != null && (toString path) == (toString rootReadmePath);

  expandNode =
    node:
      assert lib.assertMsg (nodeShapeOK node) ''
        docs: each pages node must be exactly one of
          { text = ./page.md; }
          | { title = "…"; children = [ … ]; text? /* mdSplit provenance */ }
          | { generate = "nixosOptions"; }
      '';
      if (node.generate or null) != null then
        expandGenerate node
      else if (node.children or [ ]) != [ ] then
        let
          kids = map expandNode node.children;
          # mdSplit: group has provenance `text`, children = preamble + H2 leaves.
          # Rename the preamble leaf to config.project and mark rootReadme/FIGlet
          # when provenance matches prelude.docs.rootReadme.
          kids' =
            if (node.text or null) != null && kids != [ ] then
              let
                head = builtins.head kids;
                tail = builtins.tail kids;
                isReadme = leafIsRootReadme node.text;
                head' =
                  if head.kind == "leaf" then
                    head
                    // {
                      title = project;
                      rootReadme = isReadme || (head.rootReadme or false);
                    }
                  else
                    head;
              in
              [ head' ] ++ tail
            else
              kids;
        in
        {
          kind = "group";
          title = node.title;
          children = kids';
          gapBefore = false;
          rootReadme = false;
        }
      else
        {
          kind = "leaf";
          title = if (node.title or null) != null then node.title else "";
          markdownPath = node.text;
          gapBefore = false;
          rootReadme = leafIsRootReadme node.text;
        };

  nav = map expandNode pages;

  # Walk expanded nav; assign pages/NNN.md slots and a JSON-ready tree.
  collect =
    nodes: startId:
    let
      step =
        acc: node:
        if node.kind == "group" then
          let
            sub = collect node.children acc.id;
          in
          {
            id = sub.id;
            entries = acc.entries ++ sub.entries;
            nodes = acc.nodes ++ [
              {
                kind = "group";
                title = node.title;
                children = sub.nodes;
                gapBefore = node.gapBefore or false;
              }
            ];
          }
        else
          let
            id = acc.id;
            fileName = "pages/${lib.fixedWidthNumber 3 id}.md";
            # Root README keeps its body as-authored (no forced H1 header).
            header =
              if (node.rootReadme or false) then
                ""
              else if node.title != "" then
                "# ${node.title}\n\n"
              else
                "";
          in
          {
            id = id + 1;
            entries = acc.entries ++ [
              {
                inherit id fileName header;
                path = node.markdownPath;
              }
            ];
            nodes = acc.nodes ++ [
              {
                kind = "leaf";
                title = node.title;
                markdownFile = fileName;
                gapBefore = node.gapBefore or false;
                rootReadme = node.rootReadme or false;
              }
            ];
          };
      result = lib.foldl' step
        {
          id = startId;
          entries = [ ];
          nodes = [ ];
        }
        nodes;
    in
    {
      id = result.id;
      entries = result.entries;
      nodes = result.nodes;
    };

  collected = collect nav 0;
  navForJson = collected.nodes;

  escape = lib.escapeShellArg;

  copyLines = lib.concatMapStrings
    (
      e:
      if e.header == "" then
        ''
          cp ${escape e.path} $out/${escape e.fileName}
        ''
      else
        ''
          {
            printf '%s' ${escape e.header}
            cat ${escape e.path}
          } > $out/${escape e.fileName}
        ''
    )
    collected.entries;

  metaJson = builtins.toJSON {
    inherit project colorProfile;
    palette = pal;
    nav = navForJson;
    # Relative filename resolved by the Go viewer against the config dir,
    # same convention as NavNode.markdownFile. Empty when no hero is baked
    # (e.g. empty project name) so the viewer falls back to the bold name.
    heroFile = if heroText == "" then "" else "hero.txt";
  };

  # writeText embeds path strings only; runCommand creates real store edges via cp/cat.
  metaFile = writeText "prelude-docs-meta.json" metaJson;

  # FIGlet hero rendered at build time. Pure derivation: figlet is a
  # nativeBuildInput, output is a store path, never read by Nix eval.
  heroDrv = runCommand "prelude-docs-hero"
    {
      nativeBuildInputs = [ figlet ];
    } ''
    figlet -f ${heroFont} -- ${lib.escapeShellArg heroText} > "$out"
  '';

  configBundle = runCommand "prelude-docs-config" { } ''
    mkdir -p "$out/pages"
    ${copyLines}
    cp ${metaFile} "$out/config.json"
    ${lib.optionalString (heroText != "") "cp ${heroDrv} \"$out/hero.txt\""}
  '';
in
assert lib.assertMsg (pages != [ ]) "docs: no pages configured — set prelude.docs.pages";
assert lib.assertOneOf "docs colorProfile" colorProfile [
  "auto"
  "truecolor"
  "ansi256"
];
buildGoModule {
  pname = "docs";
  version = "0.1.0";
  src = ../.;
  subPackages = [ "cmd/docs" ];
  doCheck = false;
  vendorHash = "sha256-qHpXE7MVG06KxY/2eLnqUva3/FHjAdQceH6A/5sn7mU=";
  ldflags = [
    "-s"
    "-w"
    "-X main.defaultConfigPath=${configBundle}/config.json"
  ];
  passthru = {
    config = configBundle;
  };
  # Keep configBundle in the Go drv graph even if ldflags only stringifies it.
  postConfigure = ''
    test -f ${configBundle}/config.json
  '';
  meta = {
    description = "Markdown project docs viewer";
    mainProgram = "docs";
  };
}
