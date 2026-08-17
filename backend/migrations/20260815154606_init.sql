-- +goose Up
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) UNIQUE NOT NULL,
    icon_url VARCHAR(255),
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE currencies (
    id SERIAL PRIMARY KEY,
    ticker VARCHAR(255) NOT NULL,
    full_name VARCHAR(255) NOT NULL
);

CREATE TABLE categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    user_id INTEGER,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE tags (
    id SERIAL PRIMARY KEY,
    user_id INTEGER,
    color VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE events (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    description VARCHAR(255),
    delta DECIMAL NOT NULL,
    currency_id INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (currency_id) REFERENCES currencies(id)
);

CREATE TABLE events_tags (
    event_id BIGINT NOT NULL,
    tag_id INTEGER NOT NULL,
    PRIMARY KEY (event_id, tag_id),
    FOREIGN KEY (event_id) REFERENCES events(id),
    FOREIGN KEY (tag_id) REFERENCES tags(id)
);

CREATE TABLE balances (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    amount DECIMAL NOT NULL,
    currency_id INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (currency_id) REFERENCES currencies(id)
);

-- +goose Down
DROP TABLE IF EXISTS balances;
DROP TABLE IF EXISTS events_tags;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS currencies;
DROP TABLE IF EXISTS users;
