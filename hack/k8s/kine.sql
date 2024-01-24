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