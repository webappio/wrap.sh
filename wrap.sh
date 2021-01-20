#!/usr/bin/env bash
x() {
  mkdir -p ~/.tmp || {
    echo "[wrap.sh] could not create temp directory at ~/.tmp" >&2
    return 1
  }

  curl -Ls "http://wraplocal.sh/wrapper-linux-amd64" > ~/.tmp/wrapper-linux-amd64 || {
    echo "[wrap.sh] could not download wrapper client to ~/.tmp/wrapper-linux-amd64" >&2
    return 1
  }

  chmod 700 ~/.tmp/wrapper-linux-amd64 || {
    echo "[wrap.sh] could not chmod 700 ~/.tmp/wrapper-linux-amd64" >&2
    return 1
  }
  exec ~/.tmp/wrapper-linux-amd64 "$@"
}
# wrap in a function in case network is disconnected or anything like that
x "$@"
