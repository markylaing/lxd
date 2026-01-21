# loki related test helpers.

spawn_loki() {
  # Return if loki is already set up.
  [ -e "${TEST_DIR}/loki.pid" ] && return

  go run ./apis/loki/main.go "${TEST_DIR}" &
  echo $! > "${TEST_DIR}/loki.pid"
  sleep 1
}

kill_loki() {
  [ ! -e "${TEST_DIR}/loki.pid" ] && return

  kill -9 "$(< "${TEST_DIR}/loki.pid")"
  rm -f "${TEST_DIR}/loki.*"
}
