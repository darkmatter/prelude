# Theme palettes ported from the cli-menu-design demo (lib/themes.ts).
#
# Each theme reduces the design's CSS custom properties to the tokens a
# terminal renderer needs. oklch values were converted to sRGB hex offline
# using CSS Color 4 gamut mapping (chroma reduction) for the two tokens
# that fall outside the sRGB gamut (amber.accent, paper.accent2).
#
# Token roles:
#   fg          body text                     (--foreground)
#   muted       secondary text, descriptions  (--muted-foreground)
#   dim         hints, load lines, labels     (--term-dim)
#   border      box borders, dividers — the design's --border faded to
#               50% over bg so frames stay subtle
#   accentBorder  accent-tinted border for the motd banner box
#               (term-green at 25% alpha over the card surface)
#   accent      primary accent: project name, commands, selection background
#               (--term-green / --primary)
#   accent2     secondary accent: group headers, key chips, prompt
#               (--term-amber)
#   success     positive status and healthy-state indicators
#   warning     cautionary status and degraded-state indicators
#   info        neutral/informational status indicators
#   error       destructive, required, and failed-state indicators
#   selectionFg text on accent background     (--primary-foreground)
#   bg          terminal/window background; used for inverted footer bars
#               and window fills (--background)
#   surface     card background, slightly lighter than bg (--card)
#   secondary   raised chrome: title/status bars, chips (--secondary)
{
  phosphor = {
    bg = "#0c110e";
    surface = "#131715";
    secondary = "#202622";
    fg = "#d5e2d7";
    muted = "#7d8a81";
    dim = "#5d665f";
    border = "#1a201d";
    accentBorder = "#284a2c";
    accent = "#68e371";
    accent2 = "#f9b64f";
    success = "#68e371";
    warning = "#f9b64f";
    info = "#5eb7ff";
    error = "#e6443d";
    selectionFg = "#0c110e";
  };

  # Minted: deep indigo-black surfaces with sage primary and rose secondary.
  # Ported from a CSS token set (background/card/secondary + term-green/amber).
  minted = {
    bg = "#0c0c13";
    surface = "#161623";
    secondary = "#24243f";
    fg = "#b1b1bf";
    muted = "#6a6c85";
    dim = "#4a5585";
    border = "#1d1d2f";
    accentBorder = "#3e4441";
    accent = "#f2cdcd";
    # accent = "#e979fa";
    # accent = "#b7ce99";
    # accent2 = "#e979fa";
    accent2 = "#CC99FF";
    success = "#b7ce99";
    warning = "#f2c17d";
    info = "#89b4fa";
    error = "#ee848e";
    selectionFg = "#0c0c13";
  };

  amber = {
    bg = "#110c08";
    surface = "#19120e";
    secondary = "#291f18";
    fg = "#fcbe62";
    muted = "#a97c46";
    dim = "#8d6a43";
    border = "#261c14";
    accentBorder = "#523f23";
    accent = "#ffc761";
    accent2 = "#fadc7d";
    success = "#a8c66c";
    warning = "#fadc7d";
    info = "#7fb4ca";
    error = "#e9523f";
    selectionFg = "#110c08";
  };

  solarized = {
    bg = "#0d2323";
    surface = "#162e2d";
    secondary = "#203838";
    fg = "#8b9c9d";
    muted = "#6d7e7f";
    dim = "#566768";
    border = "#1c3232";
    accentBorder = "#2e4b31";
    accent = "#75a33e";
    accent2 = "#ba9232";
    success = "#859900";
    warning = "#b58900";
    info = "#268bd2";
    error = "#db4241";
    selectionFg = "#031a1a";
  };

  nord = {
    bg = "#2e333d";
    surface = "#383d48";
    secondary = "#424853";
    fg = "#d3d8e0";
    muted = "#979fab";
    dim = "#79818d";
    border = "#3e434e";
    accentBorder = "#50615d";
    accent = "#96ce9d";
    accent2 = "#e2cc91";
    success = "#a3be8c";
    warning = "#ebcb8b";
    info = "#88c0d0";
    error = "#d55753";
    selectionFg = "#292e38";
  };

  gruvbox = {
    bg = "#282622";
    surface = "#33302b";
    secondary = "#3e3a34";
    fg = "#e2d7ba";
    muted = "#968f7b";
    dim = "#817a66";
    border = "#3a3632";
    accentBorder = "#535735";
    accent = "#b2cb52";
    accent2 = "#eebc4a";
    success = "#b8bb26";
    warning = "#fabd2f";
    info = "#83a598";
    error = "#e9483d";
    selectionFg = "#282622";
  };

  # Strict grayscale on near-black — hierarchy comes from brightness alone
  # (selection bar is near-white with dark text).
  mono = {
    bg = "#0a0a0a";
    surface = "#121212";
    secondary = "#1c1c1c";
    fg = "#e5e5e5";
    muted = "#8a8a8a";
    dim = "#5c5c5c";
    border = "#1c1c1c";
    accentBorder = "#484848";
    accent = "#ebebeb";
    accent2 = "#a3a3a3";
    success = "#ebebeb";
    warning = "#c7c7c7";
    info = "#a3a3a3";
    error = "#ffffff";
    selectionFg = "#0a0a0a";
  };

  # Ported from czxtm/apathy-theme: dark purple-tinted backgrounds
  # (panelBg/dusk/plum tiers), lavender primary accent, butterscotch
  # secondary, the theme's own muted grays and errorRed.
  apathy = {
    bg = "#0e0b13";
    surface = "#1b1629";
    secondary = "#2a2441";
    fg = "#aabbbb";
    muted = "#7d7a8b";
    dim = "#4d4a56";
    border = "#2a2630";
    accentBorder = "#463559";
    # accent = "#c792ea";
    accent = "#77f5c9";
    accent2 = "#ffcb6b";
    success = "#77f5c9";
    warning = "#ffcb6b";
    info = "#82aaff";
    error = "#e61f44";
    selectionFg = "#0e0b13";
  };

  paper = {
    bg = "#f4f2ec";
    surface = "#faf8f4";
    secondary = "#e6e4df";
    fg = "#252a27";
    muted = "#545a55";
    dim = "#6e736e";
    border = "#e2e0da";
    accentBorder = "#c3d8c1";
    accent = "#1e7729";
    accent2 = "#9f6200";
    success = "#1e7729";
    warning = "#9f6200";
    info = "#2563a6";
    error = "#cc2827";
    selectionFg = "#f7f5f1";
  };

  # Brand palette for this repo. Formerly ANSI/xterm-256 indices (fg=7, accent=212,
  # …); pinned to the standard xterm-256 hex so every terminal paints the same RGB
  # instead of rebinding 0–15 through the user's theme.
  prelude = {
    bg = "#0e0b13";
    surface = "#1b1621";
    secondary = "#8787af"; # was 103
    fg = "#c0c0c0"; # was 7 (ANSI white)
    muted = "#8787af"; # was 103
    dim = "#444444"; # was 238
    border = "#444444"; # was 238
    accentBorder = "#ff005f"; # was 197
    accent = "#ff87d7"; # was 212
    accent2 = "#c1f98e"; # was 156
    success = "#c1f98e";
    warning = "#ffd787";
    info = "#87d7ff";
    error = "#ff005f"; # was 197
    selectionFg = "#0e0b13"; # was 8; use bg for contrast on accent selection
  };
}
