
> Attribution, the original work is from  
> github.com/hairyhenderson/go-codeowners 
> This fork allow to write files and to get non global owners

# go-codeowners

A package that finds and parses [`CODEOWNERS`](https://help.github.com/articles/about-codeowners/) files.

Features:
- operates on local repos
- doesn't require a cloned repo (i.e. doesn't need a `.git` directory to be 
  present at the repo's root)
- can be called from within a repo (doesn't have to be at the root)
- will find `CODEOWNERS` files in all documented locations: the repo's root,
  `docs/`, and `.github/` (or `.gitlab/` for GitLab repos)

## Usage

```console
go get -u github.com/noandrea/go-codeowners
```

To find the owner of the README.md file:

```go
import "github.com/noandrea/go-codeowners"

func main() {
	c, _ := NewCodeowners(cwd())
	owners := c.Owners("README.md")
	for i, o := range owners {
		fmt.Printf("Owner #%d is %s\n", i, o)
	}
}
```

To generate a new codeowners file :

```go
import "github.com/noandrea/go-codeowners"

func main() {
	c, _ := EmptyCodeowners(cwd())

	err = c.AddPattern("*", []string{"@alice", "@bob"})
	CheckError(err) // this is just to shorten the example
	err = c.AddPattern("/src", []string{"@mark"})
	CheckError(err) // this is just to shorten the example
	// write the result to file
	err = c.ToFile("CODEOWNERS")
}
```

## License

[The MIT License](http://opensource.org/licenses/MIT)

Copyright (c) 2018 Dave Henderson
