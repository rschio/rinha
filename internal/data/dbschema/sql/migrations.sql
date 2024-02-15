-- Version: 1.0
-- Description: Create table clients.
CREATE TABLE IF NOT EXISTS clients(
	id INT PRIMARY KEY,
	credit_limit BIGINT NOT NULL,
	date_created TIMESTAMP NOT NULL,
	date_updated TIMESTAMP NOT NULL
);

-- Version: 1.1
-- Description: Create table transactions
CREATE TABLE IF NOT EXISTS transactions(
	id TEXT PRIMARY KEY,
	client_id INT REFERENCES clients(id),
	value BIGINT NOT NULL,
	type VARCHAR(1) NOT NULL,
	description VARCHAR(10) NOT NULL,
	date_created TIMESTAMP NOT NULL
);

-- Version: 1.2
-- Description: Insert default clients.
INSERT INTO clients (id, credit_limit, date_created, date_updated) VALUES 
(1, 100000, NOW(), NOW()),
(2, 80000, NOW(), NOW()),
(3, 1000000, NOW(), NOW()),
(4, 10000000, NOW(), NOW()),
(5, 500000, NOW(), NOW())
ON CONFLICT DO NOTHING;
