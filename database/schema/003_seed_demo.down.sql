-- Remove demo organisations inserted in 003_seed_demo.up.sql
DELETE FROM organisations
WHERE slug IN ('acme', 'testOrg');
