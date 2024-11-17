# Nabia client

Nabia is a Key/Value database implementing web technologies like HTTP and REST.

Nabia makes use of well-known and established web lingo to define basic operations.

## CRUD operations (Create, Read, Update, Delete)

### Creating data

Two methods for writing data exist. `POST` and `PUT`.

#### `POST`

`POST` is used to create data in a specific key once. Subsequent `POST` requests to the same path will fail. Therefore, `POST` is not idempotent.

For example:

```
$ ./nabia-client POST /test "test1"
Posting value "test" to key /test at localhost:5380
$ ./nabia-client POST /test "test2"
Posting value "test" to key /test at localhost:5380
expected 2xx response code, got 409 Conflict
$ ./nabia-client GET /test
Getting key /test from localhost:5380
"test1"
```

As you can see, the second `POST` attempt is rejected with HTTP error code 409, which is a standard response for these situations.

#### `PUT`

Unlike `POST`, the `PUT` verb will always overwrite data whenever there's data already present at the given key. This makes `PUT` idempotent, as repeated calls with the same data will not change the result produced.


For example:

```
$ ./nabia-client PUT /test "Hello, World!"
Putting value "Hello, World!" to key /test at localhost:5380
$ ./nabia-client PUT /test "Goodbye, World!"
Putting value "Goodbye, World!" to key /test at localhost:5380
$ ./nabia-client GET /test
Getting key /test from localhost:5380
"Goodbye, World!"
```

### Reading data

Two methods for reading data are possible: `GET` and `HEAD`.

#### `GET`

`GET` simply retrieves data from the Nabia server. If the content-type is `text/plain; charset=utf-8`, then it will print it to stdout. Otherwise, it refuses to print it.

```
$ ./nabia-client PUT /test "test123"
Putting value "test123" to key /test at localhost:5380
$ ./nabia-client GET /test
Getting key /test from localhost:5380
"test123"
```

```
$ ./nabia-client PUT /test --file $HOME/Downloads/sample.png
Putting content of file /home/x000/Downloads/sample.png to key /test at localhost:5380
$ ./nabia-client GET /test
Getting key /test from localhost:5380
Data is "image/png", not plain text, refusing to print to stdout.
```

#### `HEAD`

`HEAD` will return status code `200 OK` whenever the requested key exists:

```
$ ./nabia-client HEAD /test
Checking if key /test exists at localhost:5380
Key "/test" exists
$ ./nabia-client HEAD /non-existing-test
Checking if key /non-existing-test exists at localhost:5380
Key "/non-existing-test" does not exist
```

### Updating data

`PUT` is the only possible method to update data. The `PUT` method will always overwrite data.

### Deleting data

`DELETE` is the only method that exists to delete data given a key. Deletions are irreversible.

The Nabia client does not check if the key exists before attempting to delete it. If this is necessary, a `HEAD` must be issued prior to deletion. Deleting non-existing keys will return a `404 Not Found` error.

```
$ ./nabia-client DELETE /test
Deleting key /test from localhost:5380
$ ./nabia-client HEAD /test
Checking if key /test exists at localhost:5380
Key "/test" does not exist
$ ./nabia-client DELETE /test
Deleting key /test from localhost:5380
expected 2xx response code, got 404 Not Found
$ ./nabia-client DELETE /non-existing-test
Deleting key /non-existing-test from localhost:5380
expected 2xx response code, got 404 Not Found
```

## Other features

### Automatic `Content-Type` detection

The Nabia client has two ways of uploading data to the Nabia database. You can either supply a string via the terminal as an argument:

```
$ ./nabia-client POST /test inline_string
Posting value "test" to key /test at localhost:5380
```

Then, using `curl` we see the `Content-Type` was automatically set to UTF-8 plain text, as we supplied a string via the terminal.

```
$ curl localhost:5380/test -v 2>&1 | grep -i Content-Type
< Content-Type: text/plain; charset=utf-8
```

Now, uploading a sample image with `.png` extension, and without having to manually supply the Content-Type:

```
$ ./nabia-client PUT /test --file $HOME/Downloads/sample.png
Putting content of file /home/x000/Downloads/sample.png to key /test at localhost:5380
```

```
$ curl localhost:5380/test -v 2>&1 | grep -i Content-Type
< Content-Type: image/png
```

also gets us the expected results.
