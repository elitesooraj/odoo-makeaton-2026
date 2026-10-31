-- Permission catalog. This list must stay in sync with internal/authz.
-- Adding a permission means a new migration, never editing this one.

INSERT INTO permissions (key, category, description) VALUES
    ('users.read',           'users',         'View internal users'),
    ('users.write',          'users',         'Create and manage internal users'),
    ('audit.read',           'audit',         'View the audit log'),

    ('customers.read',       'customers',     'View customers'),
    ('customers.write',      'customers',     'Create and manage customers'),

    ('products.read',        'catalog',       'View products and variants'),
    ('products.write',       'catalog',       'Create and manage products and variants'),

    ('plans.read',           'catalog',       'View plans'),
    ('plans.write',          'catalog',       'Create and manage plans'),

    ('taxes.read',           'finance',       'View tax rules'),
    ('taxes.write',          'finance',       'Create and manage tax rules'),

    ('discounts.read',       'finance',       'View discount rules'),
    ('discounts.write',      'finance',       'Create and manage discount rules'),

    ('subscriptions.read',   'subscriptions', 'View subscriptions'),
    ('subscriptions.write',  'subscriptions', 'Create and manage subscriptions'),

    ('invoices.read',        'finance',       'View invoices'),
    ('invoices.write',       'finance',       'Issue, cancel and send invoices'),

    ('payments.read',        'finance',       'View payments'),
    ('payments.write',       'finance',       'Record payments'),

    ('reports.read',         'reports',       'View revenue and subscription reports')
ON CONFLICT (key) DO NOTHING;

-- Admin can do everything.
INSERT INTO role_permissions (role, permission_key)
SELECT 'ADMIN', key FROM permissions
ON CONFLICT DO NOTHING;

-- Staff defaults. Finance writes are intentionally absent: an admin grants
-- invoices.write / payments.write to finance staff through user_permissions.
INSERT INTO role_permissions (role, permission_key)
SELECT 'STAFF', key FROM permissions
WHERE key IN (
    'customers.read',
    'customers.write',
    'products.read',
    'plans.read',
    'taxes.read',
    'discounts.read',
    'subscriptions.read',
    'subscriptions.write',
    'invoices.read',
    'payments.read',
    'reports.read'
)
ON CONFLICT DO NOTHING;
