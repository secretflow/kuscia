#!/bin/bash
nohup prometheus --config.file=/home/config/prometheus.yml > /dev/null 2>&1 &
GRAFANA_URL="http://admin:admin@localhost:3000/api/datasources"
EXPECTED_STATUS_CODE=200
nohup grafana server --homepath=/usr/share/grafana --config=/etc/grafana/grafana.ini --packaging=docker cfg:default.log.mode=console cfg:default.paths.data=/var/lib/grafana cfg:default.paths.logs=/var/log/grafana cfg:default.paths.plugins=/var/lib/grafana/plugins cfg:default.paths.provisioning=/etc/grafana/provisioning > /dev/null 2>&1 &
while true; do
    status_code=$(curl -w "%{http_code}" -o /dev/null -s ${GRAFANA_URL})
    if [ "$status_code" -eq "$EXPECTED_STATUS_CODE" ]; then
        echo "Grafana has been started: $status_code"
        break
    else
        echo "Grafana is starting: $status_code"
    fi
    sleep 2
done

datasource_uid=$(curl -s http://admin:admin@localhost:3000/api/datasources | jq -r '.[] | select(.name == "Kuscia-monitor") | .uid')
kill $(ps -a | grep grafana | awk {'print $1'})
sed -i "s/{{Kuscia-datasource}}/${datasource_uid}/g" /var/lib/grafana/dashboards/machine.json
sed -i '1n;s/#//g' /usr/share/grafana/conf/provisioning/dashboards/sample.yaml
cp /usr/share/grafana/conf/provisioning/dashboards/sample.yaml /etc/grafana/provisioning/dashboards/sample.yaml
nohup grafana server --homepath=/usr/share/grafana --config=/etc/grafana/grafana.ini --packaging=docker cfg:default.log.mode=console cfg:default.paths.data=/var/lib/grafana cfg:default.paths.logs=/var/log/grafana cfg:default.paths.plugins=/var/lib/grafana/plugins cfg:default.paths.provisioning=/etc/grafana/provisioning > /dev/null 2>&1 &
trap "exit 0" SIGTERM
while true; do sleep 1; done
