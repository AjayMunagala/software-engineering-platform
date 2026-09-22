#!/usr/bin/env bash
set -euo pipefail
[[ "$(id -u)" == 0 ]] || { echo 'requires root only to mount isolated test image'; exit 2; }
task_root="$(mktemp -d /tmp/aegis-die505-full-XXXXXX)"
mountpoint="$task_root/mount"
mounted=false
cleanup() {
 if [[ "$mounted" == true ]]; then umount "$mountpoint" || return; fi
 case "$task_root" in /tmp/aegis-die505-full-*) rm -rf -- "$task_root";; esac
}
trap cleanup EXIT
chmod 755 "$task_root"
mkdir "$mountpoint"
truncate -s 16M "$task_root/filesystem.img"
mkfs.ext4 -q -F -m 0 "$task_root/filesystem.img"
mount -o loop,nosuid,nodev,noexec "$task_root/filesystem.img" "$mountpoint"
mounted=true
chown postgres:postgres "$mountpoint"
chmod 700 "$mountpoint"
runuser -u postgres -- env AEGIS_FAULT_SPOOL_ROOT="$mountpoint" GOPROXY=off GOSUMDB=off GOMAXPROCS=4 bash -c 'cd /mnt/d/Project_Ai/backend && /opt/aegis-go-1.26.2/bin/go test ./internal/integration/dependency -run "^TestSpool(FilesystemFull|OSFaults)$" -count=1 -v'
