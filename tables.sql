CREATE TABLE `users` (
  `uid` char(36) NOT NULL DEFAULT uuid(),
  `greed` double NOT NULL DEFAULT 0,
  `pubkey` char(64) NOT NULL DEFAULT '',
  UNIQUE KEY `users_uid_IDX` (`uid`) USING BTREE,
  UNIQUE KEY `users_pubkey_IDX` (`pubkey`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE game_vouchers (
	id BIGINT UNSIGNED auto_increment NOT NULL PRIMARY KEY,
	code varchar(48) DEFAULT '' NOT NULL
)
ENGINE=InnoDB
DEFAULT CHARSET=utf8mb4
COLLATE=utf8mb4_general_ci;

ALTER TABLE game_vouchers ADD amount FLOAT DEFAULT 1 NOT NULL;
