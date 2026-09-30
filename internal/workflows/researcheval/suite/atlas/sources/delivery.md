# Atlas Queue delivery
Atlas Queue v2 delivers at least once, so consumers may receive duplicates. On HTTP 429, clients must wait for Retry-After before retrying.
