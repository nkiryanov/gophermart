/* enable pgcrypto extension for default uuid fields */
create extension if not exists pgcrypto;

create table users (
    id uuid primary key default gen_random_uuid(),
    created_at timestamptz default now(),
    username text not null unique,
    password_hash text not null
);

create table refresh_tokens (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null,
    token varchar(255) not null unique,
    created_at timestamptz default now(),
    expires_at timestamptz not null,
    used_at timestamptz
);

create table orders (
    id uuid primary key default gen_random_uuid(),
    uploaded_at timestamptz not null default now(),
    modified_at timestamptz not null default now(),
    number varchar(255) not null unique,
    user_id uuid not null references users(id) on delete cascade,
    status varchar(32) not null check (status in ('new', 'processing', 'invalid', 'processed')),
    accrual numeric(10, 2)
);
create index idx_orders_user_id on orders(user_id);
create index idx_orders_uploaded_at on orders(uploaded_at desc);
create index idx_orders_status on orders(status);

create table transactions (
    id uuid primary key default gen_random_uuid(),
    processed_at timestamptz not null default now(),
    user_id uuid not null references users(id) on delete cascade,
    order_number varchar(255) not null,
    type varchar(32) not null check (type in ('withdraw', 'accrual')),
    amount numeric(10, 2) not null,
    constraint transactions_unique_order_and_type unique (type, order_number)
);
create index idx_transactions_user_id_type on transactions(user_id, type);

create table balances (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null unique references users(id) on delete cascade,
    current numeric(10, 2) not null,
    withdrawn numeric(10, 2) not null,
    constraint current_always_positive check (current >= 0)
);
create index idx_balances_user_id on balances(user_id);
