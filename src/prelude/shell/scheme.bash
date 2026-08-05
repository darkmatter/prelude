# shellcheck shell=bash
# Prelude color scheme for ble.sh.
#
# Nix replaces the `%prelude_*` semantic placeholders with the resolved theme
# palette. The resulting native scheme maps every face supported by the pinned
# ble.sh without depending on ble.sh's unstable internal color-alias API.

ble-import contrib/scheme/default

function ble/contrib/scheme:prelude/initialize {
  ble/contrib/scheme:default/initialize

  # Editing and validation states.
  ble-face -s region                    'fg=%prelude_fg,bg=%prelude_secondary'
  ble-face -s region_target             'fg=%prelude_selection_fg,bg=%prelude_accent'
  ble-face -s region_match              'fg=%prelude_fg,bg=%prelude_accent_border'
  ble-face -s region_insert             'fg=%prelude_selection_fg,bg=%prelude_info'
  ble-face -s disabled                  'fg=%prelude_dim'
  ble-face -s overwrite_mode            'fg=%prelude_selection_fg,bg=%prelude_warning'
  ble-face -s vbell                     'fg=%prelude_selection_fg,bg=%prelude_error'
  ble-face -s vbell_erase               'bg=%prelude_secondary'
  ble-face -s vbell_flash               'fg=%prelude_selection_fg,bg=%prelude_error,bold'
  ble-face -s prompt_status_line        'fg=%prelude_muted,bg=transparent'
  ble-face -d prelude_status_cap         'fg=%prelude_surface,bg=%prelude_shadow'

  # Shell syntax.
  ble-face -s syntax_default            'fg=%prelude_fg'
  ble-face -s syntax_command            'fg=%prelude_accent'
  ble-face -s syntax_quoted             'fg=%prelude_success'
  ble-face -s syntax_quotation          'fg=%prelude_success,bold'
  ble-face -s syntax_escape             'fg=%prelude_accent2'
  ble-face -s syntax_expr               'fg=%prelude_info'
  ble-face -s syntax_error              'fg=%prelude_selection_fg,bg=%prelude_error'
  ble-face -s syntax_varname            'fg=%prelude_warning'
  ble-face -s syntax_delimiter          'fg=%prelude_muted,bold'
  ble-face -s syntax_param_expansion    'fg=%prelude_accent2'
  ble-face -s syntax_history_expansion  'fg=%prelude_accent2,italic'
  ble-face -s syntax_function_name      'fg=%prelude_accent,bold'
  ble-face -s syntax_comment            'fg=%prelude_dim,italic'
  ble-face -s syntax_glob               'fg=%prelude_warning,bold'
  ble-face -s syntax_brace              'fg=%prelude_info,bold'
  ble-face -s syntax_tilde              'fg=%prelude_info,bold'
  ble-face -s syntax_document           'fg=%prelude_muted'
  ble-face -s syntax_document_begin     'fg=%prelude_muted,bold'

  # Command resolution.
  ble-face -s command_builtin_dot       'fg=%prelude_warning,bold'
  ble-face -s command_builtin           'fg=%prelude_warning'
  ble-face -s command_alias             'fg=%prelude_info'
  ble-face -s command_function          'fg=%prelude_accent'
  ble-face -s command_file              'fg=%prelude_success'
  ble-face -s command_keyword           'fg=%prelude_accent2'
  ble-face -s command_jobs              'fg=%prelude_error,bold'
  ble-face -s command_directory         'fg=%prelude_info,underline'
  ble-face -s command_suffix            'fg=%prelude_selection_fg,bg=%prelude_success'
  ble-face -s command_suffix_new        'fg=%prelude_selection_fg,bg=%prelude_error'
  ble-face -s cmdinfo_cd_cdpath         'fg=%prelude_info,bg=%prelude_secondary,italic'

  # Filesystem entries. LS_COLORS remains authoritative for its dedicated face.
  ble-face -s filename_directory        'fg=%prelude_info,underline'
  ble-face -s filename_directory_sticky 'fg=%prelude_selection_fg,bg=%prelude_accent,underline'
  ble-face -s filename_link             'fg=%prelude_accent2,underline'
  ble-face -s filename_orphan           'fg=%prelude_error,bold,underline'
  ble-face -s filename_executable       'fg=%prelude_success,bold,underline'
  ble-face -s filename_setuid           'fg=%prelude_selection_fg,bg=%prelude_warning,underline'
  ble-face -s filename_setgid           'fg=%prelude_selection_fg,bg=%prelude_accent2,underline'
  ble-face -s filename_other            'underline'
  ble-face -s filename_socket           'fg=%prelude_info,underline'
  ble-face -s filename_pipe             'fg=%prelude_warning,underline'
  ble-face -s filename_character        'fg=%prelude_accent2,underline'
  ble-face -s filename_block            'fg=%prelude_warning,bold,underline'
  ble-face -s filename_warning          'fg=%prelude_error,underline'
  ble-face -s filename_url              'fg=%prelude_info,underline'
  ble-face -s filename_ls_colors        'underline'

  # Variable states and command arguments.
  ble-face -s varname_array             'fg=%prelude_warning,bold'
  ble-face -s varname_empty             'fg=%prelude_error'
  ble-face -s varname_export            'fg=%prelude_accent,bold'
  ble-face -s varname_expr              'fg=%prelude_info,bold'
  ble-face -s varname_hash              'fg=%prelude_success,bold'
  ble-face -s varname_new               'fg=%prelude_success'
  ble-face -s varname_number            'fg=%prelude_info'
  ble-face -s varname_readonly          'fg=%prelude_accent2'
  ble-face -s varname_transform         'fg=%prelude_accent2,bold'
  ble-face -s varname_unset             'fg=%prelude_error'
  ble-face -s argument_option           'fg=%prelude_accent2,italic'
  ble-face -s argument_error            'fg=%prelude_selection_fg,bg=%prelude_error'

  # Completion and filtering.
  ble-face -s auto_complete             'fg=%prelude_dim,italic'
  ble-face -s menu_complete_match       'bold'
  ble-face -s menu_complete_selected    'fg=%prelude_selection_fg,bg=%prelude_accent'
  ble-face -s menu_desc_default         'fg=%prelude_muted'
  ble-face -s menu_desc_type            'ref:syntax_delimiter'
  ble-face -s menu_desc_quote           'ref:syntax_quoted'
  ble-face -s menu_filter_fixed         'fg=%prelude_accent2,bold'
  ble-face -s menu_filter_input         'fg=%prelude_selection_fg,bg=%prelude_accent2'

  return 0
}
