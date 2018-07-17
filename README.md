# tape

Tape tar:s and gzips a directory so that it always unpacks with an initial folder.


## Example

`tape ~/Sites/project1 project1.tgz`

Would take a directory like this

```
Sites/
    project1/
        file1.txt
        file2.txt
```

And when you unpacks it will look like this:

```
site/
    file1.txt
    file2.txt
```
