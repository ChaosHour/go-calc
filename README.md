# go-calc

A CLI tool to compute Google CloudSQL custom tiers based on CPU or memory values.

## Build

Compile the project:
```
make build
```

## Run

Examples:

- Calculate using CPU:
```
./bin/go-calc -cpu 24
```

- Calculate using memory:
```
./bin/go-calc -mem 6G
```
or
```
./bin/go-calc -mem 6144M
```
or
```
./bin/go-calc -mem 6144
```

- Calculate with a custom tier input:
```
./bin/go-calc -t db-custom-1-3840
```

- Bump memory to max (6.5 GB/vCPU) for an existing tier:
```
./bin/go-calc -bump-mem db-custom-4-3840
```

- Check if a recommended tier is a valid downgrade from the current tier:
```
./bin/go-calc -check-downgrade "db-custom-8-53248 db-custom-8-32000"
```

- Suggest the next valid downgrade tier from the current tier:
```
./bin/go-calc -downgrade db-custom-8-53248
```

## Validation Rules

Per [Google CloudSQL docs](https://docs.cloud.google.com/sql/docs/mysql/machine-series-overview):
- vCPUs must be 1 or an even number between 2 and 96
- Memory must be 0.9 to 6.5 GB per vCPU
- Memory must be a multiple of 256 MB
- Minimum memory: 3840 MB (3.75 GB)

## Clean

Remove built binaries:
```
make clean
```
