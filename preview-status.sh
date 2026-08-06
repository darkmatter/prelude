#!/usr/bin/env bash
#
# Throwaway preview: status line styling variants for the ble.sh status panel.
#
# Usage:
#   bash preview-status.sh          # all variants with labels
#   bash preview-status.sh <n>      # only variant n, raw (no label)
#   bash preview-status.sh <n> <w>  # variant n at width w
#
set -eu

# ── palette (prelude — this repo's brand palette from themes.nix) ────
BG=#0e0b13   SF=#1b1621   SEC=#8787af
FG=#c0c0c0   MUT=#8787af   DIM=#444444
ACC=#ff87d7   ACC2=#c1f98e  INF=#87d7ff   WRN=#ffd787   ERR=#ff005f

VARIANT=${1:-}
W=${2:-${COLUMNS:-80}}
IND=3
HINT_W=29    # "x <cmd> hint x ⇥ inline x tui"

# ── color helpers ────────────────────────────────────────────────────
h2r() { local h=${1#\#}; echo $((16#${h:0:2})) $((16#${h:2:2})) $((16#${h:4:2})); }
mx() { local a=($(h2r "$1")) b=($(h2r "$2")) p=$3
  printf '#%02x%02x%02x' \
    $((a[0]+(b[0]-a[0])*p/100)) \
    $((a[1]+(b[1]-a[1])*p/100)) \
    $((a[2]+(b[2]-a[2])*p/100)); }
tf() { local r g b; read -r r g b < <(h2r "$1"); printf '\033[38;2;%d;%d;%dm' "$r" "$g" "$b"; }
tb() { local r g b; read -r r g b < <(h2r "$1"); printf '\033[48;2;%d;%d;%dm' "$r" "$g" "$b"; }
B=$'\033[1m'; F=$'\033[2m'; R=$'\033[0m'
sp() { printf '%*s' "$1" ''; }

# ── styled hint ──────────────────────────────────────────────────────
# hint_v: $1=bg $2=key-fg $3=val-fg
hint_v() { local g=$1 kf=$2 vf=$3
  printf '%s%s%sx <cmd> '   "$(tb "$g")" "$B"   "$(tf "$kf")"
  printf '%s%s%shint '       "$R"   "$(tb "$g")" "$(tf "$vf")"
  printf '%s%s%s%sx ⇥ '      "$R"   "$(tb "$g")" "$B"   "$(tf "$kf")"
  printf '%s%s%sinline '     "$R"   "$(tb "$g")" "$(tf "$vf")"
  printf '%s%s%s%sx '        "$R"   "$(tb "$g")" "$B"   "$(tf "$kf")"
  printf '%s%s%stui'         "$R"   "$(tb "$g")" "$(tf "$vf")"
}

# hint_grad_v: $1–$6=bg stops $7=key-fg $8=val-fg
hint_grad_v() {
  local kf=$7 vf=$8
  printf '%s%s%sx <cmd> '   "$(tb "$1")" "$B"   "$(tf "$kf")"
  printf '%s%s%shint '       "$R"      "$(tb "$2")" "$(tf "$vf")"
  printf '%s%s%s%sx ⇥ '      "$R"      "$(tb "$3")" "$B"   "$(tf "$kf")"
  printf '%s%s%sinline '     "$R"      "$(tb "$4")" "$(tf "$vf")"
  printf '%s%s%s%sx '        "$R"      "$(tb "$5")" "$B"   "$(tf "$kf")"
  printf '%s%s%stui'         "$R"      "$(tb "$6")" "$(tf "$vf")"
}

# ── gradient fill: $1=width $2=start $3=end $4=fg ───────────────────
grad_fill() {
  local total=$1 s=$2 e=$3 fc=$4 segs=20
  local sw=$(( total / segs ))
  local rem=$(( total - sw * segs ))
  local i w pct c
  for (( i = 0; i < segs; i++ )); do
    w=$sw; (( i < rem )) && (( w++ ))
    pct=$(( i * 100 / (segs > 1 ? segs - 1 : 1) ))
    c=$(mx "$s" "$e" "$pct")
    printf '%s%s' "$(tb "$c")" "$(tf "$fc")"; sp "$w"
  done
  printf '%s' "$R"
}

# ── 6 gradient stops into HS[0..5] ───────────────────────────────────
grad_stops() {
  local s=$1 e=$2 i pct
  for (( i = 0; i < 6; i++ )); do
    pct=$(( i * 100 / 5 ))
    HS[i]=$(mx "$s" "$e" "$pct")
  done
}
# proportional stops: hint spans cells IND..IND+HINT_W of W
grad_stops_prop() {
  local s=$1 e=$2 i pct
  local hs0=$(( IND * 100 / W ))
  local hs1=$(( (IND + HINT_W) * 100 / W ))
  for (( i = 0; i < 6; i++ )); do
    pct=$(( hs0 + (hs1 - hs0) * i / 5 ))
    HS[i]=$(mx "$s" "$e" "$pct")
  done
}
# right stops: hint spans cells (W-HINT_W)..W
grad_stops_right() {
  local s=$1 e=$2 i pct
  local hs0=$(( (W - HINT_W) * 100 / W ))
  for (( i = 0; i < 6; i++ )); do
    pct=$(( hs0 + (100 - hs0) * i / 5 ))
    HS[i]=$(mx "$s" "$e" "$pct")
  done
}

# ════════════════════════════════════════════════════════════════════
# Row helpers
# ════════════════════════════════════════════════════════════════════

# left-aligned baseline (no indent): $1=hint_bg $2=pad_bg $3=key_fg $4=val_fg
rl0() {
  hint_v "$1" "$3" "$4"
  printf '%s%s' "$(tb "$2")" "$(tf "$MUT")"; sp $(( W - HINT_W ))
  printf '%s\n' "$R"
}

# left-aligned with gutter: $1=gutter_bg(-) $2=hint_bg $3=pad_bg $4=key_fg $5=val_fg
rl() {
  [ "$1" != "-" ] && printf '%s%s' "$(tb "$1")" "$(tf "$4")"
  sp "$IND"
  hint_v "$2" "$4" "$5"
  printf '%s%s' "$(tb "$3")" "$(tf "$MUT")"; sp $(( W - IND - HINT_W ))
  printf '%s\n' "$R"
}

# left-aligned gradient hint: $1=start $2=end $3=pad_bg $4=key_fg $5=val_fg
rlg() {
  grad_stops "$1" "$2"
  printf '%s%s' "$(tb "${HS[0]}")" "$(tf "$4")"; sp "$IND"
  hint_grad_v "${HS[@]:0:6}" "$4" "$5"
  printf '%s%s' "$(tb "$3")" "$(tf "$MUT")"; sp $(( W - IND - HINT_W ))
  printf '%s\n' "$R"
}

# left-aligned gradient hint + gradient fill: $1=start $2=end $3=key_fg $4=val_fg
rlgf() {
  grad_stops_prop "$1" "$2"
  printf '%s%s' "$(tb "${HS[0]}")" "$(tf "$3")"; sp "$IND"
  hint_grad_v "${HS[@]:0:6}" "$3" "$4"
  grad_fill $(( W - IND - HINT_W )) "${HS[5]}" "$2" "$MUT"
  printf '\n'
}

# right-aligned solid: $1=pad_bg $2=hint_bg $3=key_fg $4=val_fg
rr() {
  printf '%s%s' "$(tb "$1")" "$(tf "$3")"; sp $(( W - HINT_W ))
  hint_v "$2" "$3" "$4"
  printf '%s\n' "$R"
}

# right-aligned gradient full-width: $1=start $2=end $3=key_fg $4=val_fg
rrg() {
  local s=$1 e=$2 kf=$3 vf=$4
  local pw=$(( W - HINT_W ))
  local pep=$(( pw * 100 / W ))
  local pe; pe=$(mx "$s" "$e" "$pep")
  grad_fill "$pw" "$s" "$pe" "$kf"
  local i pct
  for (( i = 0; i < 6; i++ )); do
    pct=$(( pep + (100 - pep) * i / 5 ))
    HS[i]=$(mx "$s" "$e" "$pct")
  done
  hint_grad_v "${HS[@]:0:6}" "$kf" "$vf"
  printf '%s\n' "$R"
}

# right-aligned gradient pad + solid hint: $1=start $2=end $3=hint_bg $4=key_fg $5=val_fg
rrgp() {
  local s=$1 e=$2 hb=$3 kf=$4 vf=$5
  local pw=$(( W - HINT_W ))
  local pep=$(( pw * 100 / W ))
  local pe; pe=$(mx "$s" "$e" "$pep")
  grad_fill "$pw" "$s" "$pe" "$kf"
  hint_v "$hb" "$kf" "$vf"
  printf '%s\n' "$R"
}

# right-aligned solid pad + gradient hint: $1=pad_bg $2=start $3=end $4=key_fg $5=val_fg
rrgh() {
  printf '%s%s' "$(tb "$1")" "$(tf "$4")"; sp $(( W - HINT_W ))
  grad_stops "$2" "$3"
  hint_grad_v "${HS[@]:0:6}" "$4" "$5"
  printf '%s\n' "$R"
}

# ════════════════════════════════════════════════════════════════════
# Variants
# ════════════════════════════════════════════════════════════════════

# ── left-aligned: indent & gutter (1–4) ─────────────────────────────
r1()  { rl0 "$BG"  "$BG"  "$MUT" "$DIM"; }
r2()  { rl  "-"  "$BG"  "$BG"  "$MUT" "$DIM"; }
r3()  { rl  "$SF"  "$SF"  "$SF"  "$MUT" "$DIM"; }
r4()  { rl  "$SEC" "$SEC" "$SEC" "$MUT" "$DIM"; }

# ── left-aligned: two-tone (5–8) ─────────────────────────────────────
r5()  { rl  "$BG"  "$BG"  "$SF"  "$MUT" "$DIM"; }
r6()  { rl  "$BG"  "$BG"  "$SEC" "$MUT" "$DIM"; }
r7()  { rl  "$SF"  "$SF"  "$BG"  "$MUT" "$DIM"; }
r8()  { rl  "$ACC2" "$ACC2" "$BG"  "$BG"  "$DIM"; }

# ── left-aligned: hint fg (9–13) ─────────────────────────────────────
r9()  { rl  "$BG"  "$BG"  "$BG"  "$ACC2" "$DIM"; }
r10() { rl  "$BG"  "$BG"  "$BG"  "$ACC"  "$DIM"; }
r11() { rl  "$BG"  "$BG"  "$BG"  "$INF"  "$DIM"; }
r12() { rl  "$BG"  "$BG"  "$BG"  "$WRN"  "$MUT"; }
r13() { rl  "$SF"  "$SF"  "$SF"  "$ACC2" "$MUT"; }

# ── left-aligned: gradient hint (14–19) ──────────────────────────────
r14() { rlg "$BG"  "$SF"   "$SF"   "$MUT"  "$DIM"; }
r15() { rlg "$BG"  "$ACC2" "$ACC2" "$ACC2" "$DIM"; }
r16() { rlg "$BG"  "$ACC"  "$ACC"  "$ACC"  "$DIM"; }
r17() { rlg "$BG"  "$SEC"  "$SEC"  "$MUT"  "$DIM"; }
r18() { rlg "$BG"  "$INF"  "$INF"  "$INF"  "$DIM"; }
r19() { rlg "$ACC2" "$ACC" "$ACC"  "$ACC2" "$MUT"; }

# ── left-aligned: full-width ramp (20–23) ────────────────────────────
r20() { grad_fill "$W" "$BG"  "$SF"   "$MUT"; printf '\n'; }
r21() { grad_fill "$W" "$BG"  "$SEC"  "$MUT"; printf '\n'; }
r22() { grad_fill "$W" "$BG"  "$ACC2" "$MUT"; printf '\n'; }
r23() { grad_fill "$W" "$ACC" "$ACC2" "$MUT"; printf '\n'; }

# ── left-aligned: gradient hint + fill (24–25) ───────────────────────
r24() { rlgf "$BG"  "$ACC2" "$ACC2" "$DIM"; }
r25() { rlgf "$ACC2" "$ACC" "$ACC2" "$MUT"; }

# ── right-aligned: solid (26–28) ──────────────────────────────────────
r26() { rr  "$BG"  "$BG"  "$MUT"  "$DIM"; }
r27() { rr  "$SF"  "$SF"  "$MUT"  "$DIM"; }
r28() { rr  "$BG"  "$BG"  "$ACC2" "$DIM"; }

# ── right-aligned: two-tone (29–30) ──────────────────────────────────
r29() { rr  "$BG"  "$ACC2" "$BG"   "$DIM"; }
r30() { rr  "$SF"  "$BG"  "$MUT"  "$DIM"; }

# ── right-aligned: gradient full-width (31–33) ───────────────────────
r31() { rrg "$BG"  "$ACC2" "$ACC2" "$DIM"; }
r32() { rrg "$ACC2" "$BG"  "$MUT"  "$DIM"; }
r33() { rrg "$ACC"  "$ACC2" "$ACC2" "$MUT"; }

# ── right-aligned: gradient pad + solid hint (34–35) ─────────────────
r34() { rrgp "$BG"  "$SF"   "$SF"   "$MUT"  "$DIM"; }
r35() { rrgp "$BG"  "$ACC2" "$ACC2" "$BG"   "$DIM"; }

# ── right-aligned: solid pad + gradient hint (36) ────────────────────
r36() { rrgh "$BG" "$BG"  "$ACC2" "$ACC2" "$DIM"; }

# ════════════════════════════════════════════════════════════════════
# Dispatch
# ════════════════════════════════════════════════════════════════════

DESC=(
  [1]="baseline — bg, muted+dim keys, flush left"
  [2]="indent — bg, muted+dim, transparent gutter"
  [3]="indent + surface gutter"
  [4]="indent + secondary gutter (purple)"
  [5]="two-tone — hint bg, pad surface"
  [6]="two-tone — hint bg, pad secondary"
  [7]="two-tone — hint surface, pad bg"
  [8]="two-tone — hint accent2 (green bg), pad bg"
  [9]="hint fg — accent2 keys (green) + dim, on bg"
  [10]="hint fg — accent keys (pink) + dim, on bg"
  [11]="hint fg — info keys (blue) + dim, on bg"
  [12]="hint fg — warning keys (yellow) + muted, on bg"
  [13]="hint fg — accent2 keys + muted, on surface"
  [14]="gradient hint — bg → surface"
  [15]="gradient hint — bg → accent2 (green)"
  [16]="gradient hint — bg → accent (pink)"
  [17]="gradient hint — bg → secondary (purple)"
  [18]="gradient hint — bg → info (blue)"
  [19]="gradient hint — accent2 → accent (green to pink)"
  [20]="full-width ramp — bg → surface"
  [21]="full-width ramp — bg → secondary"
  [22]="full-width ramp — bg → accent2"
  [23]="full-width ramp — accent → accent2 (pink to green)"
  [24]="gradient hint + fill — bg → accent2"
  [25]="gradient hint + fill — accent2 → accent"
  [26]="right — bg, muted+dim"
  [27]="right — surface, muted+dim"
  [28]="right — accent2 keys (green) + dim, on bg"
  [29]="right — two-tone: pad bg, hint accent2 (green)"
  [30]="right — two-tone: pad surface, hint bg"
  [31]="right — gradient bg → accent2 (hint ends on green)"
  [32]="right — gradient accent2 → bg (hint ends on dark)"
  [33]="right — gradient accent → accent2 (pink to green)"
  [34]="right — gradient pad bg→surface, hint on surface"
  [35]="right — gradient pad bg→accent2, hint on accent2"
  [36]="right — gradient hint bg→accent2, pad on bg"
)

NUM=${#DESC[@]}

label() { printf '\n%s%s── %s ──%s\n' "$(tf "$DIM")" "$R" "$1" "$R"; }

if [ -n "$VARIANT" ]; then
  if declare -F "r$VARIANT" >/dev/null 2>&1; then
    "r$VARIANT"
  else
    printf 'Unknown variant "%s" (valid: 1–%d)\n' "$VARIANT" "$NUM" >&2
    exit 1
  fi
else
  echo
  label "palette swatches"
  printf '%s%s bg %s %s'    "$(tb "$BG")"   "$(tf "$FG")"  "$(sp 4)" "$R"
  printf '%s%s sf %s %s'    "$(tb "$SF")"   "$(tf "$FG")"  "$(sp 4)" "$R"
  printf '%s%s sec %s %s'   "$(tb "$SEC")"  "$(tf "$BG")"  "$(sp 4)" "$R"
  printf '%s%s acc %s %s'   "$(tb "$ACC")"  "$(tf "$BG")"  "$(sp 4)" "$R"
  printf '%s%s acc2 %s %s'  "$(tb "$ACC2")" "$(tf "$BG")"  "$(sp 4)" "$R"
  printf '%s%s inf %s %s'    "$(tb "$INF")"  "$(tf "$BG")"  "$(sp 4)" "$R"
  printf '%s%s wrn %s %s\n' "$(tb "$WRN")"  "$(tf "$BG")"  "$(sp 4)" "$R"
  for n in $(seq 1 "$NUM"); do
    label "$n  ${DESC[$n]}"
    "r$n"
  done
  label "usage: bash preview-status.sh <n>  — print only variant n raw"
  echo
fi
