# Customization

Select a bundled palette with `prelude.theme`:

```nix
prelude.theme = "nord";
```

Override one token without replacing the rest of the palette:

```nix
prelude.palette.accent = "#88c0d0";
```

The MOTD, menu, docs viewer, and generated Starship configuration all receive
the same resolved project identity and palette.
