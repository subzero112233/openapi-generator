package traverser

import (
	"testing"

	"github.com/jgroeneveld/trial/assert"
)

const long = `    // this is a very long line that is 
    // expected to be wrapped according 
    // to the provided maximum line len-
    // gth lorem ipsum dolor sit amet`

const short = `    // this is short enough`

func TestCommentDesc(t *testing.T) {
	str := CommentDesc("    ", "this is a very long line that is expected to be wrapped according to the provided maximum line length lorem ipsum dolor sit amet", 40)
	assert.Equal(t, long, str, "shortened comment must be correct")

	str = CommentDesc("    ", "this is short enough", 40)
	assert.Equal(t, short, str, "un-shortened comment must be correct")
}
