#!/bin/bash
#
# Copyright 2023 Ant Group Co., Ltd.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

function create_alice2bob_example_data() {
  curl -X POST 'https://127.0.0.1:8082/api/v1/domaindata/create' --header "Token: $(cat /home/kuscia/var/certs/token)" --header 'Content-Type: application/json' -d '{"domaindata_id":"alice-table","name":"alice.csv","type":"table","relative_uri":"alice.csv","domain_id":"alice","datasource_id":"default-data-source","attributes":{"description":"alice demo data"},"columns":[{"comment":"","name":"id1","type":"str"},{"comment":"","name":"age","type":"float"},{"comment":"","name":"education","type":"float"},{"comment":"","name":"default","type":"float"},{"comment":"","name":"balance","type":"float"},{"comment":"","name":"housing","type":"float"},{"comment":"","name":"loan","type":"float"},{"comment":"","name":"day","type":"float"},{"comment":"","name":"duration","type":"float"},{"comment":"","name":"campaign","type":"float"},{"comment":"","name":"pdays","type":"float"},{"comment":"","name":"previous","type":"float"},{"comment":"","name":"job_blue-collar","type":"float"},{"comment":"","name":"job_entrepreneur","type":"float"},{"comment":"","name":"job_housemaid","type":"float"},{"comment":"","name":"job_management","type":"float"},{"comment":"","name":"job_retired","type":"float"},{"comment":"","name":"job_self-employed","type":"float"},{"comment":"","name":"job_services","type":"float"},{"comment":"","name":"job_student","type":"float"},{"comment":"","name":"job_technician","type":"float"},{"comment":"","name":"job_unemployed","type":"float"},{"comment":"","name":"marital_divorced","type":"float"},{"comment":"","name":"marital_married","type":"float"},{"comment":"","name":"marital_single","type":"float"}],"vendor":"manual","author":"alice"}' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
  echo
  curl -X POST 'https://127.0.0.1:8082/api/v1/domaindata/create' --header "Token: $(cat /home/kuscia/var/certs/token)" --header 'Content-Type: application/json' -d '{"domaindata_id":"alice-dp-table","name":"alice.csv","type":"table","relative_uri":"alice.csv","domain_id":"alice","datasource_id":"default-dp-data-source","attributes":{"description":"alice demo data"},"columns":[{"comment":"","name":"id1","type":"str"},{"comment":"","name":"age","type":"float"},{"comment":"","name":"education","type":"float"},{"comment":"","name":"default","type":"float"},{"comment":"","name":"balance","type":"float"},{"comment":"","name":"housing","type":"float"},{"comment":"","name":"loan","type":"float"},{"comment":"","name":"day","type":"float"},{"comment":"","name":"duration","type":"float"},{"comment":"","name":"campaign","type":"float"},{"comment":"","name":"pdays","type":"float"},{"comment":"","name":"previous","type":"float"},{"comment":"","name":"job_blue-collar","type":"float"},{"comment":"","name":"job_entrepreneur","type":"float"},{"comment":"","name":"job_housemaid","type":"float"},{"comment":"","name":"job_management","type":"float"},{"comment":"","name":"job_retired","type":"float"},{"comment":"","name":"job_self-employed","type":"float"},{"comment":"","name":"job_services","type":"float"},{"comment":"","name":"job_student","type":"float"},{"comment":"","name":"job_technician","type":"float"},{"comment":"","name":"job_unemployed","type":"float"},{"comment":"","name":"marital_divorced","type":"float"},{"comment":"","name":"marital_married","type":"float"},{"comment":"","name":"marital_single","type":"float"}],"vendor":"manual","author":"alice"}' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
  echo
  curl https://127.0.0.1:8070/api/v1/datamesh/domaindatagrant/create -X POST -H 'content-type: application/json' -d '{"author":"alice","domaindata_id":"alice-table","grant_domain":"bob"}' \
      --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
  echo
  curl https://127.0.0.1:8070/api/v1/datamesh/domaindatagrant/create -X POST -H 'content-type: application/json' -d '{"author":"alice","domaindata_id":"alice-dp-table","grant_domain":"bob"}' \
      --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
  echo
}

function create_bob2alice_example_data() {
  curl -X POST 'https://127.0.0.1:8082/api/v1/domaindata/create' --header "Token: $(cat /home/kuscia/var/certs/token)" --header 'Content-Type: application/json' -d '{"domaindata_id":"bob-table","name":"bob.csv","type":"table","relative_uri":"bob.csv","domain_id":"bob","datasource_id":"default-data-source","attributes":{"description":"bob demo data"},"columns":[{"comment":"","name":"id2","type":"str"},{"comment":"","name":"contact_cellular","type":"float"},{"comment":"","name":"contact_telephone","type":"float"},{"comment":"","name":"contact_unknown","type":"float"},{"comment":"","name":"month_apr","type":"float"},{"comment":"","name":"month_aug","type":"float"},{"comment":"","name":"month_dec","type":"float"},{"comment":"","name":"month_feb","type":"float"},{"comment":"","name":"month_jan","type":"float"},{"comment":"","name":"month_jul","type":"float"},{"comment":"","name":"month_jun","type":"float"},{"comment":"","name":"month_mar","type":"float"},{"comment":"","name":"month_may","type":"float"},{"comment":"","name":"month_nov","type":"float"},{"comment":"","name":"month_oct","type":"float"},{"comment":"","name":"month_sep","type":"float"},{"comment":"","name":"poutcome_failure","type":"float"},{"comment":"","name":"poutcome_other","type":"float"},{"comment":"","name":"poutcome_success","type":"float"},{"comment":"","name":"poutcome_unknown","type":"float"},{"comment":"","name":"y","type":"int"}],"vendor":"manual","author":"bob"}' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
  echo
  curl -X POST 'https://127.0.0.1:8082/api/v1/domaindata/create' --header "Token: $(cat /home/kuscia/var/certs/token)" --header 'Content-Type: application/json' -d '{"domaindata_id":"bob-dp-table","name":"bob.csv","type":"table","relative_uri":"bob.csv","domain_id":"bob","datasource_id":"default-dp-data-source","attributes":{"description":"bob demo data"},"columns":[{"comment":"","name":"id2","type":"str"},{"comment":"","name":"contact_cellular","type":"float"},{"comment":"","name":"contact_telephone","type":"float"},{"comment":"","name":"contact_unknown","type":"float"},{"comment":"","name":"month_apr","type":"float"},{"comment":"","name":"month_aug","type":"float"},{"comment":"","name":"month_dec","type":"float"},{"comment":"","name":"month_feb","type":"float"},{"comment":"","name":"month_jan","type":"float"},{"comment":"","name":"month_jul","type":"float"},{"comment":"","name":"month_jun","type":"float"},{"comment":"","name":"month_mar","type":"float"},{"comment":"","name":"month_may","type":"float"},{"comment":"","name":"month_nov","type":"float"},{"comment":"","name":"month_oct","type":"float"},{"comment":"","name":"month_sep","type":"float"},{"comment":"","name":"poutcome_failure","type":"float"},{"comment":"","name":"poutcome_other","type":"float"},{"comment":"","name":"poutcome_success","type":"float"},{"comment":"","name":"poutcome_unknown","type":"float"},{"comment":"","name":"y","type":"int"}],"vendor":"manual","author":"bob"}' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
  echo
  curl https://127.0.0.1:8070/api/v1/datamesh/domaindatagrant/create -X POST -H 'content-type: application/json' -d '{"author":"bob","domaindata_id":"bob-table","grant_domain":"alice"}' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
  echo
  curl https://127.0.0.1:8070/api/v1/datamesh/domaindatagrant/create -X POST -H 'content-type: application/json' -d '{"author":"bob","domaindata_id":"bob-dp-table","grant_domain":"alice"}' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
  echo
}

domain_id=$1
if [[ ${domain_id} = "alice" ]]; then
  create_alice2bob_example_data
elif [[ ${domain_id} = "bob" ]]; then
  create_bob2alice_example_data
fi