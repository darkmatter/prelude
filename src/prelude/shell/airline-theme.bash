# shellcheck shell=bash
# Prelude vim-airline theme for ble.sh's lib/vim-airline status line.
#
# Nix replaces the `%prelude_*` semantic placeholders with the resolved theme
# palette, so the airline bar shares the MOTD, menu, docs, status-line, and
# Starship colors. Enable with:
#
#   ble-import lib/vim-airline
#   bleopt vim_airline_theme=prelude
#
# Face contract (see lib/vim-airline.sh): a/b/c are the left segments; x, y,
# and z ref c, b, and a per mode. `_replace` refs `_insert` unless set, and
# every `<name>_modified` refs `<name>`. The mode suffix comes from
# `_ble_lib_vim_airline_mode` (normal/insert/replace/visual/commandline or
# inactive), so the unsuffixed faces only seed the refs.

function ble/lib/vim-airline/theme:prelude/initialize {
  ble-face -r vim_airline_@

  # Mode block: accent per mode, readable text on top.
  ble-face -s vim_airline_a              'fg=%prelude_selection_fg,bg=%prelude_accent'
  ble-face -s vim_airline_a_insert       'fg=%prelude_selection_fg,bg=%prelude_info'
  ble-face -s vim_airline_a_replace      'fg=%prelude_selection_fg,bg=%prelude_error'
  ble-face -s vim_airline_a_visual       'fg=%prelude_selection_fg,bg=%prelude_warning'
  ble-face -s vim_airline_a_commandline  'fg=%prelude_selection_fg,bg=%prelude_accent2'
  ble-face -s vim_airline_a_inactive     'fg=%prelude_muted,bg=%prelude_surface'

  # Git block: raised chrome with the mode color carried over the foreground.
  ble-face -s vim_airline_b              'fg=%prelude_accent,bg=%prelude_secondary'
  ble-face -s vim_airline_b_insert       'fg=%prelude_info,bg=%prelude_secondary'
  ble-face -s vim_airline_b_replace      'fg=%prelude_error,bg=%prelude_secondary'
  ble-face -s vim_airline_b_visual       'fg=%prelude_warning,bg=%prelude_secondary'
  ble-face -s vim_airline_b_commandline  'fg=%prelude_accent2,bg=%prelude_secondary'
  ble-face -s vim_airline_b_inactive     'fg=%prelude_dim,bg=%prelude_surface'

  # Path block: quiet body text on the card surface.
  ble-face -s vim_airline_c              'fg=%prelude_muted,bg=%prelude_surface'
  ble-face -s vim_airline_c_inactive     'fg=%prelude_dim,bg=%prelude_surface'

  # Diagnostics.
  ble-face -s vim_airline_error          'fg=%prelude_selection_fg,bg=%prelude_error'
  ble-face -s vim_airline_warning        'fg=%prelude_selection_fg,bg=%prelude_warning'
  ble-face -s vim_airline_term           'fg=%prelude_muted,bg=%prelude_bg'

  return 0
}
