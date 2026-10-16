# Catalog refresh CI

doc_bug: The alias catalog is intentionally refreshed daily, but TestNewestListsEachFamilyOnceByHarness pinned the live catalog's Haiku version. A legitimate Haiku update also failed every hook test through its shared warmup. Use a fixed input catalog for ordering, newest-version, harness-routing, and deduplication assertions; keep live-catalog resolution coverage in TestFamilyAliasIsNewestGeneratedVersion.
