-- Player and session identifiers are now stored directly so administrators can
-- cross-reference Steam and external moderation systems. Existing databases
-- containing anonymized player_sessions are deliberately not rewritten because
-- the original identifiers cannot be recovered from one-way hashes. Rebuild
-- normalized/review data from restricted raw batches before enabling this code.
UPDATE pseudonym_policy_state
SET policy_version = 'direct-identifiers-v1',
    key_id = 'not-applicable'
WHERE singleton = true
  AND NOT EXISTS (SELECT 1 FROM player_sessions LIMIT 1);
