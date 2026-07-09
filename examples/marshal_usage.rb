# frozen_string_literal: true

# Marshal is a core module, so no require is needed.

# Serialize a Ruby object graph to a binary String, then load it back.
data = { name: "Ada", langs: ["Ruby", "Go"], year: 1815, active: true }
blob = Marshal.dump(data)
puts "dumped #{blob.bytesize} bytes (#{blob.class})"
puts "round-trips: #{Marshal.load(blob) == data}"

# Shared references are preserved: both slots load as the same object.
shared = ["x"]
pair = Marshal.load(Marshal.dump([shared, shared]))
puts "shared identity kept: #{pair[0].equal?(pair[1])}"

# The dump/load pair is the idiomatic deep-copy: mutating the copy is isolated.
orig = [1, [2, 3]]
copy = Marshal.load(Marshal.dump(orig))
copy[1] << 4
puts "deep copy — orig: #{orig.inspect}, copy: #{copy.inspect}"

# Marshal.restore is an alias for load; the format version is exposed as constants.
puts "restored: #{Marshal.restore(Marshal.dump(42))}"
puts "format version: #{Marshal::MAJOR_VERSION}.#{Marshal::MINOR_VERSION}"
