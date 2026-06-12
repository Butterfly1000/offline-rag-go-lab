-- 断点续传版单文件上传回传日志表。
-- 对应客户端 extra_upload_info 每条记录；progress=2 为请求级字段，不入本表。
-- status：0待执行 1上传中 2断点恢复中 3成功 4失败

CREATE TABLE `business_device_file_upload_log` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `account_id` int unsigned NOT NULL COMMENT '账号 ID',
  `pid` varchar(64) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL DEFAULT '' COMMENT '批次任务 ID 对应设备活动日志pid',
  `filename` varchar(200) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL DEFAULT '' COMMENT '文件名',
  `path` varchar(256) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL DEFAULT '' COMMENT '手机端本地路径',
  `key` varchar(200) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL DEFAULT '' COMMENT '七牛对象 key',
  `filesize` int unsigned NOT NULL DEFAULT '0' COMMENT '文件大小，字节',
  `uploaded_size` int unsigned NOT NULL DEFAULT '0' COMMENT '已上传大小，字节',
  `status` tinyint NOT NULL DEFAULT '0' COMMENT '0待执行 1上传中 2断点恢复中 3成功 4失败',
  `upload_time` int unsigned NOT NULL DEFAULT '0' COMMENT '客户端上报时间，Unix 秒',
  `cost_time` int unsigned NOT NULL DEFAULT '0' COMMENT '上传耗时，秒',
  `error_info` varchar(2048) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL DEFAULT '' COMMENT '错误信息',
  `file_type` varchar(12) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL DEFAULT '' COMMENT '文件类型，如 mp4、pdf',
  `cloud_type` char(1) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL DEFAULT 'q' COMMENT 'q:七牛, a:amazon, t:自建',
  `error_code` int DEFAULT NULL COMMENT '错误码，成功一般为 0',
  `conn_time` int unsigned NOT NULL DEFAULT '0' COMMENT '连接耗时，秒，沿用旧表',
  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '服务端入库时间',
  PRIMARY KEY (`id`),
  KEY `idx_account_id` (`account_id`),
  KEY `idx_pid` (`pid`),
  KEY `idx_create_time` (`create_time`),
  KEY `idx_pid_status` (`pid`, `status`),
  KEY `idx_key_ctime` (`key`, `create_time`) USING BTREE,
  KEY `idx_ctime_ctype_status_key` (`key`, `create_time`, `cloud_type`, `status`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COMMENT='企业版daemon文件上传回传日志';
