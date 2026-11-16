import unittest
from orphan_plan import classify

class OrphanPlanTests(unittest.TestCase):
    def test_managed_objects_never_become_orphans(self):
        self.assertEqual(classify('id1', 'id1', {'id1'}), 'MANAGED')
    def test_only_known_matching_unmanaged_id_is_deletable(self):
        self.assertEqual(classify('id1', 'id1', set()), 'ORPHAN_VERIFIED')
        self.assertEqual(classify('id1', None, set()), 'ORPHAN_UNVERIFIED')
    def test_missing_object_and_identity_mismatch(self):
        self.assertEqual(classify(None, 'id1', set()), 'ABSENT')
        with self.assertRaises(ValueError): classify('other-id', 'id1', set())

if __name__ == '__main__': unittest.main()
