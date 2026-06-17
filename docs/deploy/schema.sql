-- 用户表
CREATE TABLE `users` (
  `id` BIGINT UNSIGNED PRIMARY KEY auto_increment,
  `created_at` DATETIME NOT NULL,
  `updated_at` DATETIME NOT NULL,
  `deleted_at` DATETIME,
  `name` VARCHAR(50) NOT NULL comment '用户名称',
  `nick_name` VARCHAR(50) comment '用户昵称',
  `department` VARCHAR(50) comment '部门',
  `email` VARCHAR(100) NOT NULL comment '用户邮箱',
  `password` VARCHAR(255) NOT NULL comment '用户密码',
  `avatar` VARCHAR(255) comment '用户头像',
  `mobile` VARCHAR(20) comment '用户手机号',
  `status` TINYINT(1) DEFAULT 1 comment '用户状态,1可用,2禁用,3未激活'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE INDEX `idx_users_deleted_at` ON `users` (`deleted_at`);

-- 角色表
CREATE TABLE `roles` (
  `id` BIGINT UNSIGNED PRIMARY KEY auto_increment,
  `created_at` DATETIME NOT NULL,
  `updated_at` DATETIME NOT NULL,
  `deleted_at` DATETIME,
  `name` VARCHAR(50) NOT NULL,
  `description` VARCHAR(255)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE INDEX `idx_roles_deleted_at` ON `roles` (`deleted_at`);

-- 用户角色多对多关联表
CREATE TABLE `user_roles` (
  `user_id` BIGINT UNSIGNED NOT NULL,
  `role_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`user_id`, `role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 接口信息表
CREATE TABLE `apis` (
  `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
  `created_at` DATETIME NOT NULL,
  `updated_at` DATETIME NOT NULL,
  `deleted_at` DATETIME,
  `name` VARCHAR(255) NOT NULL,
  `path` VARCHAR(255) NOT NULL,
  `method` VARCHAR(10) NOT NULL,
  `description` TEXT,
  INDEX `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE INDEX `idx_apis_deleted_at` ON `apis` (`deleted_at`);

-- 角色API列表多对多关联表
CREATE TABLE `role_apis` (
  `role_id` BIGINT UNSIGNED NOT NULL,
  `api_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`role_id`, `api_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- casbin 规则表
CREATE TABLE `casbin_rule`
(
    id    bigint unsigned primary key auto_increment,
    ptype varchar(100) null COMMENT "p or g",
    v0    varchar(100) null COMMENT "subject",
    v1    varchar(100) null COMMENT "object",
    v2    varchar(100) null COMMENT "action",
    v3    varchar(100) null COMMENT "domain",
    v4    varchar(100) null COMMENT "resource",
    v5    varchar(100) null COMMENT "effect",
    constraint idx_casbin_rule
        unique (ptype, v0, v1, v2, v3, v4, v5)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

--- feishu_user
CREATE TABLE `feishu_users`
(
    uid              bigint auto_increment comment '关联users表中的用户id'
        primary key,
    created_at       datetime(3)       null,
    updated_at       datetime(3)       null,
    deleted_at       datetime(3)       null,
    avatar_big       varchar(255)      null comment '飞书用户avatar_big',
    avatar_middle    varchar(255)      null comment '飞书用户avatar_middle',
    avatar_thumb     varchar(255)      null comment '飞书用户avatar_thumb',
    avatar_url       varchar(255)      null comment '飞书用户avatar_url',
    email            varchar(255)      null comment '飞书用户email',
    employee_no      varchar(255)      null comment '飞书用户employee_no',
    en_name          varchar(255)      null comment '飞书用户en_name',
    enterprise_email varchar(255)      null comment '飞书用户enterprise_email',
    mobile           varchar(255)      null comment '飞书用户mobile',
    name             varchar(255)      null comment '飞书用户name',
    open_id          varchar(255)      null comment '飞书用户open_id',
    tenant_key       varchar(255)      null comment '飞书用户tenant_key',
    union_id         varchar(255)      null comment '飞书用户union_id',
    user_id          varchar(255)      null comment '飞书用户ID'
);

CREATE INDEX `idx_feishu_users_deleted_at` ON `feishu_users` (`deleted_at`);
-- 告警渠道表
CREATE TABLE `alert_channels` (
  `id` BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
  `created_at` DATETIME(3) NOT NULL,
  `updated_at` DATETIME(3) NOT NULL,
  `deleted_at` DATETIME(3),
  `name` VARCHAR(100) NOT NULL COMMENT '告警渠道名称(如: SRE团队钉钉群)',
  `type` VARCHAR(50) NOT NULL COMMENT '渠道类型(feishuApp/feishuBoot/webhook)',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态(0-停用, 1-启用)',
  `aggregation_status` TINYINT NOT NULL DEFAULT 0 COMMENT '聚合状态(0-停用, 1-启用)',
  `config` JSON NOT NULL COMMENT '渠道动态配置(JSON格式)',
  `description` VARCHAR(255) COMMENT '描述与备注',
  UNIQUE KEY `uni_alert_channels_name` (`name`),
  INDEX `idx_alert_channels_deleted_at` (`deleted_at`),
  INDEX `idx_alert_channels_type` (`type`),
  INDEX `idx_alert_channels_status` (`status`),
  INDEX `idx_alert_channels_aggregation_status` (`aggregation_status`),
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 告警模板表
CREATE TABLE `alert_templates` (
  `id` BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  `created_at` DATETIME(3) NOT NULL,
  `updated_at` DATETIME(3) NOT NULL,
  `deleted_at` DATETIME(3),
  `name` VARCHAR(100) NOT NULL COMMENT '模板名称',
  `receive_id_type` VARCHAR(50) NOT NULL DEFAULT '' COMMENT '接收者类型(open_id/user_id/email/chat_id/空-Webhook类无需指定)',
  `receive_id` TEXT NOT NULL COMMENT '接收者ID列表(JSON 数组)',
  `alert_channel_id` INT NOT NULL COMMENT '关联的告警渠道ID',
  `description` TEXT COMMENT '描述',
  `template` TEXT NOT NULL COMMENT '单个告警(Markdown/HTML)模板',
  `aggregation_template` TEXT COMMENT '聚合告警(Markdown/HTML)模板',
  UNIQUE KEY `uni_alert_templates_name` (`name`),
  INDEX `idx_alert_templates_deleted_at` (`deleted_at`),
  INDEX `idx_alert_templates_alert_channel_id` (`alert_channel_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 告警历史记录表
CREATE TABLE `alert_historys` (
  `id` BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
  `created_at` DATETIME(3) NOT NULL COMMENT '本条记录存入数据库的时间',
  `updated_at` DATETIME(3) NOT NULL,
  `fingerprint` VARCHAR(128) NOT NULL COMMENT '指纹',
  `starts_at` DATETIME(3) NOT NULL COMMENT '开始时间',
  `cluster` VARCHAR(128) NOT NULL DEFAULT 'default' COMMENT '租户',
  `status` VARCHAR(32) NOT NULL COMMENT '告警状态',
  `ends_at` DATETIME(3) COMMENT '告警恢复时间',
  `alert_channel_id` INT NOT NULL COMMENT '关联通道ID',
  `alert_send_record_id` INT COMMENT '关联发送记录ID和分组ID',
  `alert_silence_id` INT COMMENT '关联静默规则ID',
  `alertname` VARCHAR(255) NOT NULL,
  `severity` VARCHAR(32),
  `instance` VARCHAR(255),
  `labels` JSON COMMENT '告警标签',
  `annotations` JSON COMMENT '告警注解',
  `send_count` INT DEFAULT 1 COMMENT '发送次数',
  `is_silenced` TINYINT(1) DEFAULT 0 COMMENT '是否被静默',
  UNIQUE KEY `uk_alert_identity` (`fingerprint`, `starts_at`, `cluster`),
  INDEX `idx_status_cluster` (`status`, `cluster`),
  INDEX `idx_ends_at` (`ends_at`),
  INDEX `idx_channel_id` (`alert_channel_id`),
  INDEX `idx_send_record_id` (`alert_send_record_id`),
  INDEX `idx_history_silence_id` (`alert_silence_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 告警发送明细/日志表
CREATE TABLE `alert_send_records` (
  `id` BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  `created_at` DATETIME(3) NOT NULL,
  `updated_at` DATETIME(3) NOT NULL,
  `send_status` VARCHAR(32) NOT NULL COMMENT '发送状态(success, failed)',
  `error_message` TEXT COMMENT '如果发送失败，记录失败的报错详情(供排查)',
  `external_message_id` VARCHAR(255) COMMENT '第三方平台返回的消息ID(如飞书的 message_id)',
  INDEX `idx_send_status` (`send_status`),
  INDEX `idx_external_message_id` (`external_message_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 静默规则表
CREATE TABLE `alert_silences` (
  `id` BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  `created_at` DATETIME(3) NOT NULL,
  `updated_at` DATETIME(3) NOT NULL,
  `deleted_at` DATETIME(3),
  `cluster` VARCHAR(128) NOT NULL COMMENT '所属集群/租户',
  `type` INT COMMENT '1:指纹静默, 2:标签静默',
  `fingerprint` VARCHAR(128) COMMENT '精确匹配的指纹',
  `status` TINYINT DEFAULT 1 COMMENT '状态 0:禁用 1:启用 2:过期',
  `ends_at` DATETIME(3) NOT NULL COMMENT '结束时间',
  `starts_at` DATETIME(3) NOT NULL COMMENT '开始时间',
  `matchers` JSON NOT NULL COMMENT '匹配器集合',
  `created_by` VARCHAR(64) COMMENT '创建人',
  `comment` TEXT COMMENT '静默原因',
  INDEX `idx_alert_silences_deleted_at` (`deleted_at`),
  INDEX `idx_cluster_status_ends_starts` (`cluster`, `status`, `ends_at`, `starts_at`),
  INDEX `idx_fingerprint` (`fingerprint`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 告警抑制规则表
CREATE TABLE `alert_inhibition_rules` (
  `id` BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  `name` VARCHAR(64) COMMENT '规则名称',
  `source_matchers` JSON COMMENT '源告警(抑制者)匹配器',
  `target_matchers` JSON COMMENT '目标告警(被抑制者)匹配器',
  `equal_labels` JSON COMMENT '必须相等的标签列表',
  `status` TINYINT DEFAULT 1 COMMENT '1启用 0禁用'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 租户表
CREATE TABLE `tenants` (
  `id` BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  `created_at` DATETIME(3) NOT NULL,
  `updated_at` DATETIME(3) NOT NULL,
  `name` VARCHAR(128) NOT NULL COMMENT '租户名称',
  `label` VARCHAR(128) COMMENT '租户显示标签',
  `description` VARCHAR(255) COMMENT '租户描述'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
