#!/usr/bin/env bash
set -euo pipefail

VENV_DIR="${HOME}/.local/share/nit/venv"
NIT_PKG="$(cd "$(dirname "$(readlink -f "$0")")" && pwd)"

if [[ ! -d "${VENV_DIR}" ]]; then
    echo "nit: first run — setting up virtualenv..."
    mkdir -p "$(dirname "${VENV_DIR}")"
    python3 -m venv "${VENV_DIR}"
    "${VENV_DIR}/bin/pip" install --quiet -e "${NIT_PKG}"
    echo "nit: ready."
fi

# Reinstall if package changed (dev convenience)
if [[ "${NIT_PKG}/pyproject.toml" -nt "${VENV_DIR}/pyvenv.cfg" ]]; then
    "${VENV_DIR}/bin/pip" install --quiet -e "${NIT_PKG}"
    touch "${VENV_DIR}/pyvenv.cfg"
fi

exec "${VENV_DIR}/bin/nit" "$@"
