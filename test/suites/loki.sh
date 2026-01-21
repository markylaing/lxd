test_loki() {
  ensure_import_testimage
  ensure_has_localhost_remote "${LXD_ADDR}"
  spawn_loki

  lxc config set loki.api.url="http://$(cat "${TEST_DIR}/loki.addr")" loki.loglevel=trace loki.types=security
  curl -k "https://${LXD_ADDR}/1.0/projects"
  sleep 2
  cat "${TEST_DIR}/loki.logs"
  kill_loki
}