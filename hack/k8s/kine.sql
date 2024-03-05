--  Copyright 2023 Ant Group Co., Ltd.
--
--  Licensed under the Apache License, Version 2.0 (the "License");
--  you may not use this file except in compliance with the License.
--  You may obtain a copy of the License at
--
--    http://www.apache.org/licenses/LICENSE-2.0
--
--  Unless required by applicable law or agreed to in writing, software
--  distributed under the License is distributed on an "AS IS" BASIS,
--  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
--  See the License for the specific language governing permissions and
--  limitations under the License.

CREATE TABLE if not exists `kine` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(630) CHARACTER SET ascii DEFAULT NULL,
  `created` int(11) DEFAULT NULL,
  `deleted` int(11) DEFAULT NULL,
  `create_revision` int(11) DEFAULT NULL,
  `prev_revision` int(11) DEFAULT NULL,
  `lease` int(11) DEFAULT NULL,
  `value` mediumblob,
  `old_value` mediumblob,
  PRIMARY KEY (`id`),
  UNIQUE KEY `kine_name_prev_revision_uindex` (`name`,`prev_revision`),
  KEY `kine_name_index` (`name`),
  KEY `kine_name_id_index` (`name`,`id`),
  KEY `kine_id_deleted_index` (`id`,`deleted`),
  KEY `kine_prev_revision_index` (`prev_revision`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;