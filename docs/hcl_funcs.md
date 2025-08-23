# HCL Functions

## Standard Library
The following functions are all from the [go-cty](https://pkg.go.dev/github.com/zclconf/go-cty/cty/function/stdlib#pkg-functions), [go-cty-yaml](https://pkg.go.dev/github.com/zclconf/go-cty-yaml) and [hcl](https://pkg.go.dev/github.com/hashicorp/hcl/v2@v2.20.1/ext/tryfunc#section-readme) libraries
#### abs
`abs` returns the absolute value
```
abs(number)
```
```
abs(-1)
# 1
abs(2)
# 2
```
#### can
`can` evaluates an expression and returns a boolean if a result can be produced without any error
```
can(expr)
```
```
map = {
  myvar = "myvar"
}
can1 = can(hv.map.myVar)
# true
can2 = can(hv.map.notMyVar) 
# false
```
#### ceil
`ceil` returns the ceiling value of a given number
```
ceil(number)
```
```
ceil(1) 
# 1
ceil(1.1) 
# 2
```
#### chomp
`chomp` removes newline characters at the end of a string. 
```
chomp(string)
```
```
chomp("myVar\n")
# myVar
```
#### coalesce

`coalesce` returns the first of the given arguments that is not null. If all arguments are null, an error is produced.
All arguments must be of the same type apart from some cases
```
coalesce(any...)
```
```
coalesce(null, 2)
# 2
coalesce(null, "value")
# value
```
Use the three dots notation `...` to expand a list
```
coalesce([null, "value"]...)
# value
```
#### coalescelist
`coalescelist` takes any number of list arguments and returns the first one that isn't empty. 
```
coalescelist(list)
```
```
coalescelist([], ["value"])
# ["value"]
```
Use the three dots notation `...` when using list of lists
```
coalescelist([[], ["val1", "val2"]]...)
# ["val1", "val2"]
```
#### compact
`compact` returns a new list with any empty string elements removed.
```
compact(list)
```
```
compact(["", "val1", "val2"])
# ["val1", "val2"]
```
#### concat
`concat` takes one or more sequences (lists or tuples) and returns the single sequence that results from concatenating them together in order. 
```
concat(list, list...)
```
```
concat(["val1"], ["val2", "val3"])
# ["val1", "val2", "val3"]
```
#### contains
`contains` returns a boolean if a list contains a given value
```
contains(list, value)
```
```
contains(["val1", "val2"], "val2")
# true
```
#### csvdecode
`csvdecode` decodes a CSV-formatted string into a list of maps
```
csvdecode(string)
```
```
csvdecode("col1,col2\nv1,v2\nv3,v4")
###
[
  {
    "col1" = "v1"
    "col2" = "v2"
  },
  {
    "col1" = "v3"
    "col2" = "v4"
  }
]
```
#### distinct
`distinct` returns a new list from another by removing all duplicates
```
distinct(list)
```
```
distinct(["v1","v1","v2"])
["v1", "v2"]
```
#### element
`element` returns a single element from a given list at the given index. If index is greater than the length of the list then it is wrapped modulo the list length
```
element(list, index)
```
```
element(["val1","val2"], 1)
# val2
```

#### chunklist
`chunklist` splits a single list into fixed-size chunks, returning a list of lists. 
```
chunklist(list, size)
```
```
chunklist(["a","b"], 1)
# [["a"], ["b"]]
```
#### flatten
`flatten` takes a list and replaces any elements that are lists with a flattened sequence of the list contents. 
```
flatten(list)
```
```
flatten([["a"], ["a","b"], ["c"]])
# ["a","a","b","c"]
```
#### floor
`floor` returns the closest whole number lesser than or equal to the given value. 
```
floor(number)
```
```
floor(1)
# 1
floor(0.7)
# 0
```
#### format
`format` produces a string representation of zero or more values using a format string similar to the "printf" function in C.
[Verbs details](https://pkg.go.dev/github.com/zclconf/go-cty/cty/function/stdlib#Format)
```
format(format, values)
```
```
format("Hello %s", "world")
# Hello world
``` 
#### formatdate
`formatdate` reformats a timestamp given in RFC3339 syntax into another time syntax defined by a given format string.
[Syntax details](https://pkg.go.dev/github.com/zclconf/go-cty/cty/function/stdlib#FormatDate)
```
formatdate(string, timestampString)
```
```
formatdate("MMM DD YYYY", "2024-01-01T00:12:00Z")
# Jan 01 2024
```

#### formatlist
`formatlist` does the same as `format` but for a list of strings
```
formatlist(formatString, values...)
```
```
formatlist("%s", ["Hello", "World"])
###
[
  "Hello",
  "World"
]

formatlist("%s %s", "hello", ["World", "You"])
###
[
  "hello World",
  "hello You",
]
```
#### indent
`indent` adds a given number of spaces to the beginnings of all but the first line in a given multi-line string. 
```
indent(number, string)
```

```
indent(4, "hello,\nWorld\n!")
###
hello
    World
    !
```
#### int
`int` removes the fractional component of the given number returning an integer representing the whole number component, rounding towards zero.

```
int(number)
```
```
int(6.2)
# 6
```

#### join
`join` concatenates together the string elements of one or more lists with a given separator. 
```
join(listOfStrings, separator)
```
```
join(" ", ["hello", "world"])
# hello world
```

#### jsondecode
`jsondecode` parses the given JSON string and, if it is valid, returns the value it represents. 
```
jsonencode(string)
```
Example :
```
jsonencode({"hello"="world"})
# {"hello": "world"}
```

#### jsonencode
`jsonencode` returns a JSON serialization of the given value. 
```
jsondecode(string)
```
Example :
```
jsondecode("{\"hello\": \"world\"}")
# { hello = "world" }
```
#### keys
`keys` takes a map and returns a sorted list of the map keys.

```
keys(map)
```
 
```
keys({val1=1, val2=2, val3=3})
# ["val1","val2","val3"]
```
#### length
`length` returns the number of elements in the given __collection__. 
See `strlen` for strings
```
length(list)
```

```
length([1,2,3])
# 3
```
#### log
`log` returns returns the logarithm of a given number in a given base. 
```
log(number, base)
```

```
log(1, 10)
# 0
```
#### lookup
`lookup` performs a dynamic lookup into a map. There are three required arguments, inputMap and key, plus a defaultValue, which is a value to return if the given key is not found in the inputMap. 
```
lookup(inputMap, key, defaultValue)
```

```
map = { "luke" = "skywalker"}
lookup(hv.maptest, "luke", "none")
# skywalker
lookup(hv.maptest, "leia", "none")
# none
```
#### lower
`lower` is a Function that converts a given string to lowercase.
```
lower(string)
```

```
lower("HELLO world")
# hello world
```
#### max
`max` returns the maximum number from the given numbers. 
```
max(numbers)
```

```
max(1,128,70)
# 128

```
#### merge
`merge` takes an arbitrary number of maps and returns a single map that contains a merged set of elements from all of the maps.
```
merge(maps)
```
```
merge({a="1"}, {a=[1,2], c="world"}, {d=40})
# { a = [1,2], c = "world", d = 40}

```
#### min
`min` returns the minimum number from the given numbers. 
```
min(numbers)
```
```
min(1,128,70)
# 1
```
#### parseint
`parseint` parses a string argument and returns an integer of the specified base. 
```
parseint(string, base)
```
```
parseint("190", 10)
# 190
parseint("11001", 2)
# 25
```
#### pow
`pow` returns the logarithm of a given number in a given base. 
```
pow(number, power)
```

```
pow(1, 10)
# 1
pow(3, 12)
# 531441
```
#### range
`range` creates a list of numbers by starting from the given starting value, then adding the given step value until the result is greater than or equal to the given stopping value. Each intermediate result becomes an element in the resulting list.
```
range(startingNumber, stoppingNumber, stepNumber)
```

```
range(1, 10, 3)
# [1, 4, 7]
```
#### regex
`regex` is a function that extracts one or more substrings from a given string by applying a regular expression pattern, describing the first match. 
The return type depends on the composition of the capture groups (if any) in the pattern:

    If there are no capture groups at all, the result is a single string representing the entire matched pattern.
    If all of the capture groups are named, the result is an object whose keys are the named groups and whose values are their sub-matches, or null if a particular sub-group was inside another group that didn't match.
    If none of the capture groups are named, the result is a tuple whose elements are the sub-groups in order and whose values are their sub-matches, or null if a particular sub-group was inside another group that didn't match.
    It is invalid to use both named and un-named capture groups together in the same pattern.

If the pattern doesn't match, this function returns an error. To test for a match, call `regexall` and check if the length of the result is greater than zero. 
```
regex(pattern, string)
```

```
regex("[0-9]+", "v1.2.3")
# 1
```
#### regexall
`regexall` is similar to Regex but it finds all of the non-overlapping matches in the given string and returns a list of them.

The result type is always a list, whose element type is deduced from the pattern in the same way as the return type for Regex is decided.

If the pattern doesn't match at all, this function returns an empty list. 
```
regexall(pattern, string)
```

```
regexall("[0-9]+", "v1.2.3")
# [1 2 3]
```
#### setintersection
`setintersection` returns a new set containing the elements that exist in all of the given sets, which must have element types that can all be converted to some common type using the standard type unification rules. If conversion is not possible, an error is returned. 
```
setintersection(sets...)
```

```
setintersection(["val1", "val2"], ["val1", "val3"], ["val1", "val2"])
# ["val1"]
```
#### setproduct
`setproduct` computes the Cartesian product of sets or sequences.
```
setproduct(sets...)
```
```
setproduct(["host1", "host2"], ["stg.domain", "prod.domain"])
### 
[
  [
    "host1",
    "stg.domain"
  ],
  [
    "host2",
    "stg.domain"
  ],
  [
    "host1",
    "prod.domain"
  ],
  [
    "host2",
    "prod.domain"
  ],
]
``` 
#### setsubtract
`setsubtract` returns a new set containing the elements from the first set that are not present in the second set. The sets must have element types that can both be converted to some common type using the standard type unification rules. If conversion is not possible, an error is returned. 
```
setsubtract(sets...)
```
```
setsubtract(["a", "b", "c"], ["a", "b"])
### 
["c"]
``` 
#### setunion
`setunion` returns a new set containing all of the elements from the given sets, which must have element types that can all be converted to some common type using the standard type unification rules. If conversion is not possible, an error is returned. 
```
setunion(sets...)
```

```
setunion(["a", "b"], ["b", "c"], ["a", "d"])
### 
["a", "b", "c", "d"]
``` 
#### signum
`signum` determines the sign of a number, returning a number between -1 and 1 to represent the sign. 
```
signum(number)
```
```
signum(-182)
# -1
``` 
#### slice
`slice` extracts some consecutive elements from within a list.
startIndex is inclusive, endIndex is exclusive
```
slice(list, startIndex, endIndex)
```
```
slice([{"a" = "b"}, {"c" = "d"}, , {"e" = "f"}], 1, 1)
# []
slice([{"a" = "b"}, {"c" = "d"}, {"e" = "f"}], 1, 2)
# [{"c" = "d"}]
``` 
#### sort
`sort` re-orders the elements of a given list of strings so that they are in ascending lexicographical order.
```
sort(list)
```
```
sort(["1", "h", "r", "p", "word"])
# ["1", "h", "p", "r", "word"]
``` 
#### split
`split` divides a given string by a given separator, returning a list of strings containing the characters between the separator sequences. 
```
split(separatorString, string)
```
```
split(".", "host.domain")
# ["host", "domain"]
``` 
#### strlen
`strlen` is a Function that returns the length of the given string in characters. 
```
strlen(string)
```
```
strlen("yes")
# 3
``` 
#### strrev
`strrev` is a Function that reverses the order of the characters in the given string.
```
strrev(string)
```
```
strrev("yes")
# "sey"
``` 
#### substr
`substr` is a Function that extracts a sequence of characters from another string and creates a new string. 
```
substr(string, offsetNumber, length)
```
```
substr("host.domain", 0, 4)
# "host"
``` 
#### timeadd
`timeadd` adds a duration to a timestamp, returning a new timestamp.
Only units "inferior" or equal to `h` are supported.
The duration can be negative.
```
substr(timestamp, duration)
```
```
timeadd("2024-01-01T00:00:00Z", "-2600h10m")
# 2023-09-14T15:50:00Z
``` 
#### trim
`trim` removes the specified characters from the start and end of the given string.
```
trim(string, string)
```
```
trim("Can you do that ? Yes ?", "?")
# "Can you do that ? Yes"
``` 
#### trimprefix
`trimprefix` removes the specified prefix from the start of the given string. 
```
trimprefix(stringToTrim, trimmingString)
```
```
trimprefix("please, do it", "please, ")
# "do it"
``` 
#### trimspace
`trimspace` removes any space characters from the start and end of the given string.
```
trimspace(string)
```
```
trimspace("   Hello World   ")
# "Hello World"
``` 
#### trimsuffix
`trimsuffix` removes the specified suffix from the end of the given string. 
```
trimsuffix(stringToTrim, trimmingString)
```
```
trimsuffix("Hello World", " World")
# "Hello"
``` 
#### try
`try` is a variadic function that tries to evaluate all of is arguments in sequence until one succeeds, in which case it returns that result, or returns an error if none of them succeed. 
```
try(expressions...)
```
```
values {
  map = {
    hello = "you"
    world = "us"
  }
  try(hv.map.do_not_exist, hv.map.world)
}
# "us"
``` 
#### upper
`upper` is a Function that converts a given string to uppercase. 
```
upper(string)
```
```
upper("up")
# "UP"
``` 
#### values
`values` returns a list of the map values, in the order of the sorted keys. This function only works on flat maps.
```
values(map)
```
```
values({"a" = 1,"b" = 2})
# [1, 2]
``` 
#### yamldecode
`yamldecode` parses the given JSON string and, if it is valid, returns the value it represents. 
```
yamldecode(string)
```
```
yamldecode("hello: world\narray: [1, 2, 3]")
###
{
  array = [1, 2, 3]
  hello = "world"
}
``` 
#### yamlencode
`yamlencode` returns a JSON serialization of the given value.
```
yamlencode({array = [1, 2, 3], hello = "world"})
```

```
yamlencode({array = [1, 2, 3], hello = "world"})
###
"array":
- 1
- 2
- 3
"hello": "world"
``` 
#### zipmap
`zipmap` constructs a map from a list of keys and a corresponding list of values.
The lenght of each list must be equal
```
zipmap(keysList, valuesList)
```
```
zipmap(["key1", "key2"], ["val1", "val2"])
###
{
  "key1" = "val1"
  "key2" = "val2"
}
``` 