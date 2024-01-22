FROM openanolis/anolisos:8.8

ENV TZ=Asia/Shanghai

ARG ROOT_DIR="/home"

RUN yum install -y jq https://dl.grafana.com/oss/release/grafana-10.0.0-1.x86_64.rpm && \
    yum clean all

RUN mkdir -p /home/config
RUN mkdir -p /var/lib/grafana/dashboards/
COPY scripts/templates/kuscia-monitor-datasource.yaml /etc/grafana/provisioning/datasources/
COPY scripts/templates/grafana-dashboard-machine.json /var/lib/grafana/dashboards/machine.json
COPY scripts/templates/grafana-dashboard-network.json /var/lib/grafana/dashboards/network.json
COPY build/dockerfile/prometheus /bin
COPY scripts/deploy/init_kuscia_monitor.sh /home
ENV PATH="${PATH}:${ROOT_DIR}/bin:/bin/aux"
WORKDIR ${ROOT_DIR}
RUN chmod +x /home/init_kuscia_monitor.sh
CMD ["/home/init_kuscia_monitor.sh"]
ENTRYPOINT ["/bin/bash", "--"]
