# Public TypeScript dispatch contracts. Import alongside the other flake checks;
# fixtures use the real Prelude.make API and libprelude, not a stand-in Bun script.
{
  pkgs,
  lib,
  localFlake,
}: let
  typescript = import ./typescript.nix {inherit pkgs lib;};
  entrypoint = "ops/prelude.ts";
  source = pkgs.writeText "typescript-dispatch-prelude.ts" ''
    import { Command, Prelude } from "${../ts}/src/index.ts";

    const revision = "published";
    const report = (event: string, extra = {}) => console.log(JSON.stringify({
      event, revision, cwd: process.cwd(), argv: process.argv.slice(2), ...extra,
    }));
    report("boot");
    await Prelude.make({
      project: "typescript-dispatch-fixture",
      dispatcher: "x",
      menu: { just: { enable: false }, scripts: { enable: false } },
      commands: {
        "ts-function": Command.make({
          description: "TypeScript function dispatch sentinel",
          args: [
            { token: "--label", required: true },
            { token: "<target>", required: true },
          ],
          run: (args, context) => {
            report("run", { args, key: context.key, raw: context.argv });
            return 0;
          },
        }),
        "ts-status": Command.make({
          description: "TypeScript exit status sentinel",
          run: () => { report("status"); return 37; },
        }),
      },
    }).main();
  '';
  root = pkgs.runCommand "typescript-dispatch-published-root" {} ''
    mkdir -p "$out/ops"
    cp ${source} "$out/${entrypoint}"
  '';
  emptyRoot = pkgs.runCommand "typescript-dispatch-empty-root" {} ''
    mkdir -p "$out"
  '';
  configuration.prelude = {
    project = "typescript-dispatch";
    inherit root;
    typescript.entrypoint = entrypoint;
    menu = {
      enable = true;
      just.enable = false;
      scripts.enable = false;
    };
    motd.enable = false;
    prompt.enable = false;
    workspace.enable = false;
    # This must neither leak into the TS catalogue nor produce a PATH wrapper.
    commands."nix-only" = {
      description = "Nix command that must not be exposed in TypeScript mode";
      exec = "echo NIX_DISPATCH_MUST_NOT_RUN; exit 99";
    };
  };
  evaluate = fixturePkgs: overrides:
    localFlake.lib.evalModule fixturePkgs (lib.recursiveUpdate configuration overrides);
  packages = (evaluate pkgs {}).packages;
  missingPackages = (evaluate pkgs {prelude.root = emptyRoot;}).packages;
  drvPaths = packages: lib.mapAttrs (_: package: package.drvPath) packages;
  rejects = value:
    !(builtins.tryEval (
      builtins.deepSeq (
        drvPaths
        (evaluate pkgs {
          prelude.typescript.entrypoint = value;
        }).packages
      )
      true
    )).success;
  invalidEntrypoints = [
    ""
    "/ops/prelude.ts"
    "../ops/prelude.ts"
    "ops/../prelude.ts"
  ];
  # Disabled menu must not force Bun just because an entrypoint was configured.
  disabled = evaluate (pkgs // {bun = throw "disabled menu evaluated Bun";}) {
    prelude.menu.enable = false;
  };
  legacy = evaluate pkgs {prelude.typescript.entrypoint = null;};
  manifest = pkgs.writeText "typescript-dispatch-manifest.json" (
    builtins.toJSON {
      inherit entrypoint root;
      shell = "${packages.prelude-shell}/bin";
      published = "${packages.prelude-menu}/bin";
      missingPublished = "${missingPackages.prelude-menu}/bin";
      bun = "${pkgs.bun}/bin/bun";
    }
  );
  probe = pkgs.writeText "typescript-dispatch-check.py" ''
    import json
    import os
    from pathlib import Path
    import shutil
    import subprocess
    import sys
    import tempfile

    manifest = json.loads(Path(sys.argv[1]).read_text())
    entrypoint = manifest["entrypoint"]
    env = dict(os.environ, HOME=os.environ["TMPDIR"], NO_COLOR="1", TERM="dumb")

    def run(command, cwd):
        result = subprocess.run(
            [str(word) for word in command], cwd=cwd, env=env,
            stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, text=True, timeout=30,
            # Prevent the picker from opening the build caller's /dev/tty.
            start_new_session=True,
        )
        print(f"{command!r} in {str(cwd)!r}: {result.returncode}", flush=True)
        return result

    def events(result):
        return [json.loads(line) for line in result.stdout.splitlines() if line.startswith('{"event":')]

    def check(condition, message, result=None):
        if not condition:
            if result is not None:
                print(result.stdout, file=sys.stderr)
                print(result.stderr, file=sys.stderr)
            raise AssertionError(message)

    def boot(result, revision, cwd, argv):
        observed = events(result)
        check(bool(observed), "entrypoint did not run", result)
        check(observed[0] == {
            "event": "boot", "revision": revision,
            "cwd": str(Path(cwd).resolve()), "argv": argv,
        }, f"wrong TypeScript source, cwd, or argv: {observed[0]!r}", result)
        return observed

    def fixture(path, revision):
        path.mkdir(parents=True)
        shutil.copytree(manifest["root"], path, dirs_exist_ok=True)
        # Store source permissions must not make the checkout fixture immutable.
        path.chmod(0o755)
        for copied in path.rglob("*"):
            copied.chmod(0o755 if copied.is_dir() else 0o644)
        file = path / entrypoint
        file.write_text(file.read_text().replace('"published"', json.dumps(revision)))
        return path

    words = ["ts-function", "--label", "two words; $(must-not-run)", "--", "--literal target"]
    with tempfile.TemporaryDirectory(prefix="typescript-dispatch-", dir=os.environ["TMPDIR"]) as temporary:
        base = Path(temporary)
        checkout = fixture(base / "checkout with spaces", "checkout")
        nested = checkout / "deep" / "directory"
        nested.mkdir(parents=True)
        unrelated = base / "unrelated"
        unrelated.mkdir()
        shell = Path(manifest["shell"])
        published = Path(manifest["published"])

        for bin_dir in [shell, published]:
            check(not (bin_dir / "nix-only").exists(), "Nix command wrapper leaked into TypeScript mode")

        def dispatch(bin_dir, cwd, revision, root):
            result = run([bin_dir / "x", *words], cwd)
            check(result.returncode == 0, "function dispatch failed", result)
            observed = boot(result, revision, root, words)
            check(len(observed) == 2 and observed[1] == {
                "event": "run", "revision": revision,
                "cwd": str(Path(root).resolve()), "argv": words,
                "args": {"label": words[2], "target": words[4]},
                "key": "ts-function", "raw": words[1:],
            }, "function arguments lost quoting or --", result)

            result = run([bin_dir / "x", "ts-status"], cwd)
            check(result.returncode == 37, "function exit status was not propagated", result)
            check([event["event"] for event in boot(result, revision, root, ["ts-status"])] == ["boot", "status"],
                  "exit status did not come from the TS function", result)

            result = run([bin_dir / "x", "--list"], cwd)
            check(result.returncode == 0, "--list failed", result)
            check(len(boot(result, revision, root, ["--list"])) == 1, "--list ran a command", result)
            check("ts-function" in result.stdout and "ts-status" in result.stdout,
                  "--list did not use the TS catalogue", result)
            check("nix-only" not in result.stdout and "NIX_DISPATCH_MUST_NOT_RUN" not in result.stdout,
                  "Nix catalogue leaked into TS --list", result)

            # Real non-TTY picker behavior, compared with directly running the
            # same API; no fake picker and no terminal-error wording dependency.
            direct = run([manifest["bun"], Path(root) / entrypoint], root)
            check(direct.returncode != 0 and bool(direct.stderr), "non-TTY picker unexpectedly succeeded", direct)
            for executable in ["x", "menu"]:
                result = run([bin_dir / executable], cwd)
                check(len(boot(result, revision, root, [])) == 1, "picker ran a command", result)
                check(result.returncode == direct.returncode and result.stderr == direct.stderr,
                      f"{executable} did not launch the same no-args TS picker", result)

            for arguments in [["--list"], ["ts-status"], ["--"]]:
                result = run([bin_dir / "menu", *arguments], cwd)
                check(result.returncode != 0 and bool(result.stderr), "menu accepted arguments", result)
                check(not events(result), "menu forwarded arguments to the TS app", result)

        dispatch(shell, nested, "checkout", checkout)
        # A nearer independent project wins even inside an outer checkout.
        inner = fixture(checkout / "inner project", "nearest")
        inner_nested = inner / "deep"
        inner_nested.mkdir()
        dispatch(shell, inner_nested, "nearest", inner)

        # Reuse the exact same built launcher after an edit: no store snapshot.
        file = checkout / entrypoint
        file.write_text(file.read_text().replace('"checkout"', '"fresh"'))
        dispatch(shell, nested, "fresh", checkout)

        # Published commands ignore both a conflicting checkout and a directory
        # with no entrypoint, and always chdir to the configured immutable root.
        dispatch(published, nested, "published", manifest["root"])
        dispatch(published, unrelated, "published", manifest["root"])

        for bin_dir, cwd in [
            (shell, unrelated),
            (Path(manifest["missingPublished"]), nested),
        ]:
            for executable in ["x", "menu"]:
                result = run([bin_dir / executable], cwd)
                check(result.returncode != 0, "missing entrypoint unexpectedly succeeded", result)
                check(entrypoint in result.stderr, "missing entrypoint error omitted the configured path", result)
                check(not events(result), "missing entrypoint fell back to another project's app", result)
  '';
in {
  typescript-dispatch-config = assert lib.assertMsg (builtins.all rejects invalidEntrypoints)
  "TypeScript entrypoints must be nonempty project-relative paths without parent traversal";
  assert lib.assertMsg (!(disabled.packages ? prelude-menu)) "disabled menu must not publish a TypeScript dispatcher";
  assert lib.assertMsg (
    legacy.config.prelude.typescript.entrypoint == null && legacy.packages ? prelude-menu
  ) "a null TypeScript entrypoint must preserve the Nix menu";
    pkgs.runCommand "typescript-dispatch-config"
    {
      # Force package generation with the poison Bun, not just the menu gate.
      disabledShell = disabled.packages.prelude-shell.drvPath;
    }
    ''
      touch "$out"
    '';

  typescript-dispatch =
    pkgs.runCommand "typescript-dispatch"
    {
      nativeBuildInputs = [pkgs.python3];
      PRELUDE_LIB = typescript.libraryPath;
    }
    ''
      python ${probe} ${manifest}
      touch "$out"
    '';
}
