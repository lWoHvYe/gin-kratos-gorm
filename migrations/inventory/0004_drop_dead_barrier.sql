-- Remove the obsolete per-service barrier table; dtm_barrier.barrier is owned by 02-dtm.sql.
DROP TABLE IF EXISTS dtm_barrier;
