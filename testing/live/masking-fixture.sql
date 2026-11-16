-- Applies only to the new owned tf_live_test fixture database.
DO $$
DECLARE c text;
BEGIN
 FOREACH c IN ARRAY ARRAY['name','email','ssn','phone','cc','aadhar','first4','last4','redact','hash','disable'] LOOP
  EXECUTE format('ALTER TABLE public.customers ADD COLUMN IF NOT EXISTS %I text', 'mask_' || c);
 END LOOP;
END $$;
UPDATE public.customers SET
 mask_name='Synthetic Person', mask_email='fixture@example.invalid',
 mask_ssn='111-22-3333', mask_phone='5551234567', mask_cc='4111111111111111',
 mask_aadhar='123456789012', mask_first4='ABCDEFGHIJKL', mask_last4='ABCDEFGHIJKL',
 mask_redact='synthetic-redact-value', mask_hash='synthetic-hash-value',
 mask_disable='synthetic-unmasked-control'
WHERE id=1;
SELECT count(*) AS nonempty_fixture_rows FROM public.customers WHERE mask_email IS NOT NULL;
