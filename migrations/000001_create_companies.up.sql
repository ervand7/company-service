CREATE TABLE IF NOT EXISTS companies
(
    id                  uuid PRIMARY KEY,
    name                text        NOT NULL UNIQUE CHECK (char_length(name) <= 15),
    description         text        NOT NULL DEFAULT '' CHECK (char_length(description) <= 3000),
    amount_of_employees integer     NOT NULL CHECK (amount_of_employees >= 0),
    registered          boolean     NOT NULL,
    type                text        NOT NULL CHECK (type IN ('Corporations', 'NonProfit', 'Cooperative',
                                                             'Sole Proprietorship')),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
