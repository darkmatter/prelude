# MOTD banner option namespace

## Goal

Group the MOTD banner's related configuration under `motd.banner` and move the banner tagline out of the shared Prelude namespace.

## Public API

The banner configuration becomes:

```nix
prelude.motd.banner = {
  badge = "▓▒░";
  label = "development shell";
  tagline = "everything you need to build, test & ship";
  border = {
    width = 1;
    foreground = null;
    rounded = true;
  };
  statusItems = [ ];
};
```

Remove these old options without compatibility aliases:

- `prelude.tagline`
- `prelude.motd.badge`
- `prelude.motd.bannerLabel`
- `prelude.motd.border`
- `prelude.motd.statusItems`

`prelude.project` remains shared. `prelude.motd.description` remains outside `banner` because it renders beneath the banner box.

## Internal configuration

Use the same nested `banner` shape for direct `lib.mkMotd` consumers. Direct calls remain component-level rather than wrapping values in `motd`:

```nix
prelude.lib.mkMotd deps {
  project = "acme-web";
  banner = {
    tagline = "everything you need to build, test & ship";
    border.width = 2;
  };
  description.text = "A reproducible development environment.";
}
```

Defaults live at `defaults.motd.banner`. The flake-parts module forwards `cfg.motd.banner` unchanged to `mkMotd`; it does not maintain a separate flat internal API.

The renderer merges supplied banner values with `defaults.motd.banner`, including a nested merge for `banner.border`, and reads the heading from `banner.badge`, `banner.label`, and `banner.tagline`. Status chips come from `banner.statusItems`.

## Migration scope

Update all repository-owned module configurations, direct generator calls, demos, comments, and README documentation to use the nested paths. This is intentionally a breaking API change.

## Validation

Add an evaluation check proving every nested module option resolves to its configured value and that a partial `banner.border` setting retains the other border defaults. Update direct-render checks to use the nested generator API. Run formatting and the flake checks to verify module evaluation, generated shell validation, and rendered output remain correct.
