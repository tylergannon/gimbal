# Catalog refresh CI

doc_bug: The alias catalog is intentionally refreshed daily, but TestNewestListsEachFamilyOnceByHarness pinned the live catalog's Haiku version. A legitimate Haiku update also failed every hook test through its shared warmup.
correction: Tyler asked to delete the version-pinning test. Deleted it; TestFamilyAliasIsNewestGeneratedVersion already checks newest-alias and explicit-version resolution against whatever the generated catalog contains. No replacement fixture is needed.
