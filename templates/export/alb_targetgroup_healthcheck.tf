health_check {
    healthy_threshold = ${HEALTHCHECK_HEALTHYTHRESHOLD}
    unhealthy_threshold = ${HEALTHCHECK_UNHEALTHYTHRESHOLD}
    protocol = "${HEALTHCHECK_PROTOCOL}"
    path = "${HEALTHCHECK_PATH}"
    interval = ${HEALTHCHECK_INTERVAL}
    matcher = "${HEALTHCHECK_MATCHER}"
    timeout = ${HEALTHCHECK_TIMEOUT}
  }
