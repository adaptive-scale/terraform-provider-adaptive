\set ON_ERROR_STOP on
DO $$
BEGIN
 IF current_database() <> 'tf_live_test' THEN RAISE EXCEPTION 'wrong fixture database'; END IF;
 IF (SELECT count(*) FROM public.customers) <> 1 THEN RAISE EXCEPTION 'fixture row missing'; END IF;
 IF (SELECT count(*) FROM public.out_of_scope) <> 1 THEN RAISE EXCEPTION 'privileged baseline scope missing'; END IF;
END $$;
SELECT 'terraform_fixture_smoke_passed' AS assertion;
