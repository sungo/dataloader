dataloader implements the Facebook data loader pattern to optimize data
fetches, using the power of generics

[![builds.sr.ht status](https://builds.sr.ht/~sungo/dataloader.svg)](https://builds.sr.ht/~sungo/dataloader?)

# wat

When Facebook gave the world GraphQL, it also gave us a data loading problem.

Consider this GraphQL schema:

~~~
type Query {
    user(id: ID!): User
    users(ids: [ID!]!): [User!]!
}

type User {
    id: ID!
    name: String
}
~~~

Under the hood, you'll typically have a resolver that loads an individual
user by ID, used in this case by the 'user' query. The 'users' query probably
also makes use of the 'user' loader, under the hood. And here's the problem.
If someone makes a call to 'users' and asks for 40 user ids, your GraphQL code
will make 40 database calls. They ask for 80000, you'll be making 80000 database
calls. That's... bad.

So what do? That's where this library comes in. The dataloader wraps those
requests in goroutines that batch the calls and run them concurrently.
By default, the max batch size is 1000 so those 80000 requests become 80, loaded
in parallel. (Typically, your fetch function will do something like a `select *
from wat where id in (?)`)

~~~
func fetch(keys []string) (map[string]MyThing, error) {
    // ....
}

func main() {
    loader, err := dataloader.New(fetch)
    result, err := loader.Load("wat")

    results, err := loader.LoadMany("wat", "foo")
    for key, res := range results {}
}
~~~

This library also works pretty well as the second stage for a search process.
The first stage would identify the relevant records, return the ids, and the
second stage would use this library to load the full records in parallel.

    ids := DoSearch("my * search string")
    results, err := loader.LoadMany(ids...)

See examples/simple/main.go for a fully formed example.

# Generics

The examples all use strings but since we're using generics, the keys are
anything comparable and values can be anything at all. The `fetch` function
needs to return a map of those keys to those values.

# Deduplication

Under the hood, we deduplicate the keys to be fetched in a particular batch.
If the loader is asked to fetch the same ID fifty times, that ID will only get
fetched once and handed back fifty times.

Let's say you're fetching the details of the place a user lives. The query will
ask for details on Los Angeles millions of times, but the library will only
bother to fetch it once. You are freed from having to deduplicate that yourself.

One super big caveat here. If the multiple requests span multiple batches,
the data will get fetched multiple times. We do no caching. So if batch size is
1000 and the details for Los Angeles are requested 10k times, the data will be
fetched 10 times. It's a little unintuitive but 10 is still better than 10k.

# Shared Loader

One way to use this library is to build a loader for each query. A GraphQL query
for 'users' happens, we create a loader and do the thing. This helps us, sure,
but we can take it further.

dataloader does no internal caching and is thread-safe. To gain the benefits of
batched loading and deduplication for everyone, you can create a loader at start
time, stick it in a context, and use it wherever. All loads from everywhere will
be batched together and benefits conferred.

This does require a little dance with type instantiation.

See examples/context/main.go for an example.

# LICENSE

MIT License

Copyright (c) 2023 sungo

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
