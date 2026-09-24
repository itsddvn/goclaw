-- Migration 99 reconciles two histories that share version 98. A rollback
-- cannot determine which archive/grant tables already existed, and must not
-- drop archived messages or retroactively grant Contact access to users.
DO $$ BEGIN
    RAISE EXCEPTION 'migration 99 is not safely reversible; restore a pre-upgrade backup';
END $$;
