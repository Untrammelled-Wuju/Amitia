(module
  (memory (export "memory") 1)

  (global $heap_ptr (mut i32) (i32.const 4096))

  (func $amitia_alloc (export "amitia_alloc") (param $size i32) (result i32)
    (local $ptr i32)
    global.get $heap_ptr
    local.set $ptr
    global.get $heap_ptr
    local.get $size
    i32.add
    global.set $heap_ptr
    local.get $ptr
  )

  (func $amitia_dealloc (export "amitia_dealloc") (param $ptr i32) (param $size i32)
  )

  (func $amitia_invoke (export "amitia_invoke") (param $input_ptr i32) (param $input_len i32) (result i64)
    (local $result_ptr i32)
    (local $i i32)
    (local $byte i32)

    global.get $heap_ptr
    local.set $result_ptr
    global.get $heap_ptr
    local.get $input_len
    i32.add
    global.set $heap_ptr

    i32.const 0
    local.set $i
    (block $copy_done
      (loop $copy_loop
        local.get $i
        local.get $input_len
        i32.ge_s
        br_if $copy_done

        local.get $input_ptr
        local.get $i
        i32.add
        i32.load8_u
        local.set $byte

        local.get $result_ptr
        local.get $i
        i32.add
        local.get $byte
        i32.store8

        local.get $i
        i32.const 1
        i32.add
        local.set $i
        br $copy_loop
      )
    )

    local.get $result_ptr
    i64.extend_i32_u
    i64.const 32
    i64.shl
    local.get $input_len
    i64.extend_i32_u
    i64.or
  )
)
