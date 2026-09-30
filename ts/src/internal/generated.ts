// Generated from src/prelude/themes.nix and src/prelude/defaults.nix.
// Do not edit: run `x ts:sync` after changing either file.

export const themes = {
  "amber": {
    "accent": "#ffc761",
    "accent2": "#fadc7d",
    "accentBorder": "#523f23",
    "bg": "#110c08",
    "border": "#261c14",
    "dim": "#8d6a43",
    "error": "#e9523f",
    "fg": "#fcbe62",
    "info": "#7fb4ca",
    "muted": "#a97c46",
    "secondary": "#291f18",
    "selectionFg": "#110c08",
    "success": "#a8c66c",
    "surface": "#19120e",
    "warning": "#fadc7d"
  },
  "apathy": {
    "accent": "#77f5c9",
    "accent2": "#ffcb6b",
    "accentBorder": "#463559",
    "bg": "#0e0b13",
    "border": "#2a2630",
    "dim": "#4d4a56",
    "error": "#e61f44",
    "fg": "#aabbbb",
    "info": "#82aaff",
    "muted": "#7d7a8b",
    "secondary": "#2a2441",
    "selectionFg": "#0e0b13",
    "success": "#77f5c9",
    "surface": "#1b1629",
    "warning": "#ffcb6b"
  },
  "gruvbox": {
    "accent": "#b2cb52",
    "accent2": "#eebc4a",
    "accentBorder": "#535735",
    "bg": "#282622",
    "border": "#3a3632",
    "dim": "#817a66",
    "error": "#e9483d",
    "fg": "#e2d7ba",
    "info": "#83a598",
    "muted": "#968f7b",
    "secondary": "#3e3a34",
    "selectionFg": "#282622",
    "success": "#b8bb26",
    "surface": "#33302b",
    "warning": "#fabd2f"
  },
  "minted": {
    "accent": "#f2cdcd",
    "accent2": "#CC99FF",
    "accentBorder": "#3e4441",
    "bg": "#0c0c13",
    "border": "#1d1d2f",
    "dim": "#4a5085",
    "error": "#ee848e",
    "fg": "#979db5",
    "info": "#89b4fa",
    "muted": "#6a6c85",
    "secondary": "#24243f",
    "selectionFg": "#0c0c13",
    "success": "#b7ce99",
    "surface": "#161623",
    "warning": "#f2c17d"
  },
  "mono": {
    "accent": "#ebebeb",
    "accent2": "#a3a3a3",
    "accentBorder": "#484848",
    "bg": "#0a0a0a",
    "border": "#1c1c1c",
    "dim": "#5c5c5c",
    "error": "#ffffff",
    "fg": "#e5e5e5",
    "info": "#a3a3a3",
    "muted": "#8a8a8a",
    "secondary": "#1c1c1c",
    "selectionFg": "#0a0a0a",
    "success": "#ebebeb",
    "surface": "#121212",
    "warning": "#c7c7c7"
  },
  "nord": {
    "accent": "#96ce9d",
    "accent2": "#e2cc91",
    "accentBorder": "#50615d",
    "bg": "#2e333d",
    "border": "#3e434e",
    "dim": "#79818d",
    "error": "#d55753",
    "fg": "#d3d8e0",
    "info": "#88c0d0",
    "muted": "#979fab",
    "secondary": "#424853",
    "selectionFg": "#292e38",
    "success": "#a3be8c",
    "surface": "#383d48",
    "warning": "#ebcb8b"
  },
  "paper": {
    "accent": "#1e7729",
    "accent2": "#9f6200",
    "accentBorder": "#c3d8c1",
    "bg": "#f4f2ec",
    "border": "#e2e0da",
    "dim": "#6e736e",
    "error": "#cc2827",
    "fg": "#252a27",
    "info": "#2563a6",
    "muted": "#545a55",
    "secondary": "#e6e4df",
    "selectionFg": "#f7f5f1",
    "success": "#1e7729",
    "surface": "#faf8f4",
    "warning": "#9f6200"
  },
  "phosphor": {
    "accent": "#68e371",
    "accent2": "#f9b64f",
    "accentBorder": "#284a2c",
    "bg": "#0c110e",
    "border": "#1a201d",
    "dim": "#5d665f",
    "error": "#e6443d",
    "fg": "#d5e2d7",
    "info": "#5eb7ff",
    "muted": "#7d8a81",
    "secondary": "#202622",
    "selectionFg": "#0c110e",
    "success": "#68e371",
    "surface": "#131715",
    "warning": "#f9b64f"
  },
  "prelude": {
    "accent": "#ff87d7",
    "accent2": "#c1f98e",
    "accentBorder": "#ff005f",
    "bg": "#0e0b13",
    "border": "#444444",
    "dim": "#444444",
    "error": "#ff005f",
    "fg": "#c0c0c0",
    "info": "#87d7ff",
    "muted": "#8787af",
    "secondary": "#8787af",
    "selectionFg": "#0e0b13",
    "success": "#c1f98e",
    "surface": "#1b1621",
    "warning": "#ffd787"
  },
  "solarized": {
    "accent": "#75a33e",
    "accent2": "#ba9232",
    "accentBorder": "#2e4b31",
    "bg": "#0d2323",
    "border": "#1c3232",
    "dim": "#566768",
    "error": "#db4241",
    "fg": "#8b9c9d",
    "info": "#268bd2",
    "muted": "#6d7e7f",
    "secondary": "#203838",
    "selectionFg": "#031a1a",
    "success": "#859900",
    "surface": "#162e2d",
    "warning": "#b58900"
  }
} as const;

export const defaults = {
  "colorProfile": "truecolor",
  "menu": {
    "execute": true,
    "height": 16,
    "just": {
      "enable": false,
      "group": "just",
      "justfile": null
    },
    "maxWidth": 80,
    "placeholder": "type to filter commands…"
  },
  "motd": {
    "align": "center",
    "background": false,
    "border": false,
    "clearScreen": true,
    "description": {
      "text": ""
    },
    "env": [],
    "gettingStarted": {
      "commandsLabel": "commands",
      "examplesLabel": "examples",
      "heading": "Getting Started"
    },
    "header": {
      "background": false,
      "status": {},
      "statusHint": {
        "layout": "inline",
        "links": []
      },
      "tagline": {
        "align": "left",
        "layout": "stack",
        "text": ""
      }
    },
    "links": [],
    "margin": {
      "bottom": null,
      "left": null,
      "minHeight": 40,
      "right": null,
      "top": null,
      "x": 4,
      "y": 2
    },
    "maxWidth": 80,
    "padding": {
      "minHeight": 0,
      "x": 6,
      "y": 4
    },
    "title": {
      "align": "center",
      "style": "spine",
      "text": null
    },
    "verticalAlign": "center",
    "width": "full"
  },
  "theme": "minted"
} as const;
