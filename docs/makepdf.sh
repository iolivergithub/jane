#!/bin/sh
# makepdf.sh — build the whole Jane documentation as one PDF, via pandoc and
# LaTeX (XeLaTeX) with the Eisvogel template.
#
#   docs/makepdf.sh [output.pdf]          (default: docs/jane-documentation.pdf)
#
# The chapters are the pages linked from docs/contents.md, in that order, so a
# page added to contents.md is in the PDF too; the changelog comes last.
#
# Needs (Debian/Ubuntu):
#   apt install pandoc texlive-xetex texlive-latex-extra texlive-fonts-recommended \
#               texlive-fonts-extra lmodern fonts-dejavu librsvg2-bin curl
#   - pandoc 3.x (Eisvogel 3.x for pandoc >= 3.5, Eisvogel 2.5.0 for older pandoc,
#     chosen automatically); texlive-latex-extra has the LaTeX packages Eisvogel uses
#   - librsvg2-bin (rsvg-convert) turns the SVG diagrams into something LaTeX can use
#   - the Eisvogel template: taken from $EISVOGEL if set (path to eisvogel.latex),
#     else from pandoc's template directory, else downloaded once into it from
#     https://github.com/Wandmalfarbe/pandoc-latex-template
#
# Environment:
#   EISVOGEL=/path/to/eisvogel.latex   use this template file
#   PDF_ENGINE=lualatex                another Unicode-capable engine (default xelatex)
#   VERBOSE=1                          pandoc --verbose (look for the first "! LaTeX Error")
#   KEEP=1                             keep the temporary directory (prepared sources, filter)
#
# Links between pages become links inside the PDF; links to files outside docs/
# (the changelog, READMEs, code) point to the repository on GitHub.

set -eu

DOCS=$(cd "$(dirname "$0")" && pwd)
ROOT=$(dirname "$DOCS")
OUT=${1:-"$DOCS/jane-documentation.pdf"}
ENGINE=${PDF_ENGINE:-xelatex}
REPO_URL=https://github.com/iolivergithub/jane/blob/main

die() { echo "makepdf: $*" >&2; exit 1; }

command -v pandoc >/dev/null 2>&1 || die "pandoc not found (apt install pandoc)"
command -v "$ENGINE" >/dev/null 2>&1 || die "$ENGINE not found (apt install texlive-xetex)"
command -v rsvg-convert >/dev/null 2>&1 || echo "makepdf: WARNING: rsvg-convert not found (apt install librsvg2-bin); SVG images will fail" >&2

# --- the Eisvogel template ---------------------------------------------------
if [ -n "${EISVOGEL:-}" ]; then
    [ -f "$EISVOGEL" ] || die "EISVOGEL=$EISVOGEL is not a file"
    TEMPLATE=$EISVOGEL
else
    TPLDIR="$(pandoc --version | sed -n 's/^User data directory: //p')/templates"
    [ "$TPLDIR" != "/templates" ] || TPLDIR="${XDG_DATA_HOME:-$HOME/.local/share}/pandoc/templates"
    TEMPLATE="$TPLDIR/eisvogel.latex"
    if [ ! -f "$TEMPLATE" ]; then
        command -v curl >/dev/null 2>&1 || die "Eisvogel not installed and curl not found; set EISVOGEL"
        mkdir -p "$TPLDIR"
        # Eisvogel 3.x needs pandoc 3.5 or later (the split default template);
        # older pandoc (e.g. Debian 12: 3.1.3, Debian 13: 3.1.11) needs Eisvogel 2.5.0.
        PV=$(pandoc --version | sed -n '1s/^pandoc[^0-9]*\([0-9]*\)\.\([0-9]*\).*/\1 \2/p')
        set -- $PV
        if [ "${1:-0}" -gt 3 ] || { [ "${1:-0}" -eq 3 ] && [ "${2:-0}" -ge 5 ]; }; then
            echo "makepdf: pandoc $1.$2: downloading the latest Eisvogel into $TPLDIR"
            DL=$(mktemp -d)
            curl -fsSL -o "$DL/Eisvogel.tar.gz" \
                https://github.com/Wandmalfarbe/pandoc-latex-template/releases/latest/download/Eisvogel.tar.gz \
                || die "download failed; fetch eisvogel.latex yourself and set EISVOGEL"
            tar -xzf "$DL/Eisvogel.tar.gz" -C "$DL" 2>/dev/null
            found=$(find "$DL" -name eisvogel.latex | head -n 1)
            [ -n "$found" ] || die "eisvogel.latex not in the download; set EISVOGEL"
            cp "$found" "$TEMPLATE"
            rm -rf "$DL"
        else
            echo "makepdf: pandoc $1.$2 is older than 3.5: downloading Eisvogel 2.5.0 into $TPLDIR"
            curl -fsSL -o "$TEMPLATE" \
                https://raw.githubusercontent.com/Wandmalfarbe/pandoc-latex-template/v2.5.0/eisvogel.tex \
                || { rm -f "$TEMPLATE"; die "download failed; fetch eisvogel.tex (2.5.0) yourself and set EISVOGEL"; }
        fi
    fi
fi

# --- work directory ----------------------------------------------------------
WORK=$(mktemp -d)
if [ "${KEEP:-}" = 1 ]; then echo "makepdf: working in $WORK"; else trap 'rm -rf "$WORK"' EXIT; fi

# The chapters: every page linked from contents.md, in order, then the changelog.
PAGES=$(grep -o '([A-Za-z0-9_-]*\.md[)#]' "$DOCS/contents.md" | tr -d '()#' | awk '!seen[$0]++')
[ -n "$PAGES" ] || die "no pages found in contents.md"

# Each page gets an anchor named after its file, so links to "page.md" have a target.
INPUTS=""
for p in $PAGES; do
    [ -f "$DOCS/$p" ] || { echo "makepdf: WARNING: $p is linked from contents.md but missing" >&2; continue; }
    id=$(echo "$p" | sed 's/\.md$//')
    { printf '[]{#page-%s}\n\n' "$id"; cat "$DOCS/$p"; printf '\n\n'; } > "$WORK/$p"
    INPUTS="$INPUTS $WORK/$p"
done
if [ -f "$ROOT/CHANGELOG.md" ]; then
    { printf '[]{#page-CHANGELOG}\n\n'; cat "$ROOT/CHANGELOG.md"; } > "$WORK/CHANGELOG.md"
    INPUTS="$INPUTS $WORK/CHANGELOG.md"
fi

# Lua filter: page.md#anchor -> #anchor, page.md -> #page-<page>, ../path -> GitHub.
cat > "$WORK/links.lua" <<EOF
local repo = "$REPO_URL"
function Link(el)
  local t = el.target
  if t:match("^%a[%w+.-]*:") or t:match("^#") then return el end   -- URL or local anchor
  local file, anchor = t:match("^([^#]*)#?(.*)$")
  if file:match("^%.%./") then
    el.target = repo .. "/" .. file:gsub("^%.%./", "") .. (anchor ~= "" and ("#" .. anchor) or "")
  elseif file:match("^[%w_-]+%.md$") then
    if file == "CHANGELOG.md" or file == "contents.md" then
      el.target = "#page-" .. file:gsub("%.md$", "")
    elseif anchor ~= "" then
      el.target = "#" .. anchor
    else
      el.target = "#page-" .. file:gsub("%.md$", "")
    end
  else
    el.target = repo .. "/docs/" .. file .. (anchor ~= "" and ("#" .. anchor) or "")
  end
  return el
end
EOF

VERSION=$(tail -n 1 "$ROOT/etc/debbuild/versions.txt" 2>/dev/null | awk '{print $1}')
cat > "$WORK/metadata.yaml" <<EOF
title: "Jane"
subtitle: "Remote attestation engine — documentation"
author: [ "github.com/iolivergithub/jane" ]
date: "Version ${VERSION:-unknown}, built $(date +%Y-%m-%d)"
lang: en-GB
book: true
titlepage: true
titlepage-rule-height: 2
toc: true
toc-own-page: true
toc-depth: 2
mainfont: "DejaVu Sans"   # has ✓ and arrows; DejaVu Serif lacks ✓
sansfont: "DejaVu Sans"
monofont: "DejaVu Sans Mono"
fontsize: 10pt
geometry: [ "a4paper", "margin=2.2cm" ]
colorlinks: true
table-use-row-colors: true
code-block-font-size: \\scriptsize
footnotes-pretty: true
EOF

set -- \
    --from=gfm+attributes+bracketed_spans \
    --resource-path="$DOCS:$ROOT" \
    --metadata-file="$WORK/metadata.yaml" \
    --lua-filter="$WORK/links.lua" \
    --template="$TEMPLATE" \
    --top-level-division=chapter \
    --pdf-engine="$ENGINE" \
    -o "$OUT"
[ "${VERBOSE:-}" = 1 ] && set -- --verbose "$@"

echo "makepdf: $(echo "$INPUTS" | wc -w) files -> $OUT"
# shellcheck disable=SC2086  # INPUTS is a list of paths without spaces (mktemp)
pandoc "$@" $INPUTS
echo "makepdf: done"
