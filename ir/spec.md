# Spec for the IR
- subset of orlang
- 3AC?
- SSA form?
- Externs
- No methods
- No interfaces or generics
- Arch independent structs. Padding etc will happen in codegen
- Struct prop extracting based on index
- Tuples as structs

# types
- structs with no prop names {int64,float64}. Referred by index
- int32, int64, int16, int8, uint32, uint64, uint16, uint8,
- float32, float64
- bool
- ptr<type>

# instructions
- var name : type
- call name([arg]) : type
- return var
- alloc type
- free
- store ptr, var, ?index
- load ptr, var, ?index
- br
- br_cond


# Orlang code

```
struct Foo {
  var x : int32 = 1
  var y : int32 = 2
}

fn foobar(x : int32, y : int32) => Foo {
  if (x > 10) {
    return Foo{10, y}
  }

  return Foo{x, y}
}

```


# Generated IR

```
type Foo {int32, int32}

fn foobar(%x : int32, %y : int32) : ptr<Foo> {
  %temp0 = 10 : int32
  %temp1 = %x > %temp0 : bool

  br_cond %temp1, label0, label1

label0:
  %temp2 = 10 : int32
  %temp3 = alloc Foo : ptr<Foo>

  store %temp3, %temp2
  store %temp3, %y, 1

  return %temp3 : ptr<Foo>

label1:
  %temp4 = alloc Foo : ptr<Foo>

  store %temp3, %x
  store %temp3, %y, 1

  return %temp4 : ptr<Foo>
}

```
