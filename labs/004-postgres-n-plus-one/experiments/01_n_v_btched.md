## N+1 vs Batched

With all the setup steps run (and thus both analysis and postgres containers running and the network delay set up), let's time the N+1 queries:
```
time docker exec -e PGPASSWORD=postgres lab-postgres psql -h 127.0.0.1 -U postgres -d labdb -o /dev/null \   
  -f /tmp/n_plus_one.sql

docker exec -e PGPASSWORD=postgres lab-postgres psql -h 127.0.0.1 -U postgres  0.02s user 0.05s system 4% cpu 1.569 total
```

And the batched version:
```
time docker exec -e PGPASSWORD=postgres lab-postgres psql -h 127.0.0.1 -U postgres -d labdb -o /dev/null -c \
  "SELECT * FROM books WHERE author_id = ANY(ARRAY[$IDS]);"
docker exec -e PGPASSWORD=postgres lab-postgres psql -h 127.0.0.1 -U postgres  0.03s user 0.02s system 30% cpu 0.172 total
```

The difference in timing is clear (though if we hadn't introduced any form of network delay, it would have been minimal).
