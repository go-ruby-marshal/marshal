# Examples

Runnable pure-Ruby usage of the `Marshal` module, verified under the [rbgo](https://github.com/go-embedded-ruby) interpreter.

```sh
rbgo examples/marshal_usage.rb
```

| File | Shows |
| --- | --- |
| `marshal_usage.rb` | `Marshal.dump`/`load` round-trip, preserved shared references, the deep-copy idiom, the `restore` alias, and the `MAJOR_VERSION`/`MINOR_VERSION` constants |
