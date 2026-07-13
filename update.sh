#!/bin/bash

# CompanyMaps 9 deployment script.
#
# Cross-compiles the Go binary for Linux, installs it under /opt/cmaps, installs
# the systemd unit and (re)starts the service. Run this ON THE LINUX SERVER only.
# The build is fully vendored, so the server needs no network access for modules,
# but it does need a Go toolchain to compile. (To build elsewhere, run the
# GOOS=linux build line below and copy build/cmaps_linux + config.json to the
# server's /opt/cmaps.)

set -euo pipefail

appname="cmaps"
applabel="CompanyMaps"

# ---- colored output helpers ----
RED='\033[1;31m'
GRN='\033[1;32m'
YLW='\033[1;33m'
NC='\033[0m'

info() { echo -e "${YLW}$1${NC}"; }
ok()   { echo -e "${GRN}$1${NC}"; }

die() {
  echo
  echo -e "${RED}========================================================${NC}"
  echo -e "${RED}  ERROR: $1${NC}"
  echo -e "${RED}  Aborting update. The running service was not changed${NC}"
  echo -e "${RED}  unless a later step already modified it.${NC}"
  echo -e "${RED}========================================================${NC}"
  echo
  exit 1
}

check() {
  if [ "$1" -ne 0 ]; then
    die "$2"
  fi
}

info "Building executable .."
rm -rf build || die "Could not remove old build directory"
GOOS=linux GOARCH=amd64 go build -mod=vendor -o "build/${appname}_linux" ./cmd/cmaps
check $? "go build failed"

if [ ! -s "build/${appname}_linux" ]; then
  die "Build reported success but build/${appname}_linux is missing or empty"
fi
ok "Build succeeded"

if [ -f "/etc/systemd/system/${appname}.service" ]; then
  info "Stopping service .."
  systemctl stop "${appname}"
  check $? "Failed to stop ${appname} service"
fi

info "Updating executable .."
mkdir -p "/opt/${appname}"
check $? "Could not create /opt/${appname}"
mv "build/${appname}_linux" "/opt/${appname}/${appname}_linux"
check $? "Could not move new binary into /opt/${appname}"

# Install a default config.json on first deployment (never overwrite an existing one).
if [ ! -f "/opt/${appname}/config.json" ] && [ -f "config.json" ]; then
  info "Installing default config.json .."
  cp config.json "/opt/${appname}/config.json"
  check $? "Could not install config.json"
fi

info "Updating service .."
mkdir -p "/var/log/${appname}"
check $? "Could not create /var/log/${appname}"
cp template.service "${appname}.service"
check $? "Could not copy template.service"
sed -i -e "s/APPNAME/${appname}/g" "${appname}.service"
check $? "Could not substitute APPNAME in service file"
sed -i -e "s/APPLABEL/${applabel}/g" "${appname}.service"
check $? "Could not substitute APPLABEL in service file"
sudo mv "${appname}.service" "/etc/systemd/system/${appname}.service"
check $? "Could not install service file"
sudo systemctl daemon-reload
check $? "systemctl daemon-reload failed"

info "Enabling service .."
systemctl enable "${appname}"
check $? "Failed to enable ${appname} service"
echo

info "Starting service .."
systemctl start "${appname}"
check $? "Failed to start ${appname} service"
echo

sleep 1
if ! systemctl is-active --quiet "${appname}"; then
  die "${appname} service is not active after start (check: journalctl -u ${appname})"
fi

systemctl status "${appname}" --no-pager
echo
ok "Update complete: ${applabel} is running."
echo
