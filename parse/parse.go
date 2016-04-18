/*
The parse package parses byte arrays into valid sgf GameTrees. Or it fails.
*/
package parse

/*
TODO: proper encoding support
TODO: error messages should use the positional information returned by the item.pos field to indicate exactly where the problem was encountered
*/

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/apiarian/sgf"
)

// Pos is a position within a buffer
type Pos int

func Parse(data []byte) (collection []*sgf.GameTree, warnings []error, err error) {
	warnings = []error{}
	err = nil

	// looking for the encoding property
	var encodingName string
	key := []byte("CA[")
	if key_i := bytes.Index(data, key); key_i != -1 {
		start_i := key_i + len(key)
		if close_i := bytes.IndexByte(data[start_i:], ']'); close_i != -1 {
			end_i := start_i + close_i
			encodingName = string(data[start_i:end_i])
		}
	}
	if encodingName == "" {
		warnings = append(warnings, fmt.Errorf("Did not find a CA property. Assuming the UTF-8 encoding"))
		encodingName = "UTF-8"
	}
	if !strings.EqualFold(encodingName, "UTF-8") {
		// TODO: add proper encoding support.
		warnings = append(warnings, fmt.Errorf("The encoding specified in the CA property (%s) is not currently supported. Going to try to use UTF-8 instead", encodingName))
		encodingName = "UTF-8"
	}

	l := lex("data", data)
	collection, err = parseCollection(l)

	return
}

func parseCollection(l *lexer) (collection []*sgf.GameTree, err error) {
	collection = []*sgf.GameTree{}

	for lastItem := range l.items {
		switch lastItem.typ {
		case itemOpenParen:
			// a ( means we are at the beginning of an upper level GameTree
			var gt *sgf.GameTree
			gt, err = parseGameTree(l)
			if err != nil {
				return
			}
			collection = append(collection, gt)
			// parseGameTree consumes the closing ) of a GameTree so we can go around
			// again and look for another top level GameTree if there are no errors
		case itemEOF:
			return
		case itemError:
			err = fmt.Errorf("%s", lastItem.val)
			return
		default:
			// the only things allowed at the outer level of a file are GameTrees, so
			// anything other than an ( or an EOF is an error
			err = fmt.Errorf("parse: only GameTrees are allowed at the top level")
			return
		}
	}
	err = fmt.Errorf("parse: lexer channel closed unexpectedly")
	return
}

func parseGameTree(l *lexer) (gt *sgf.GameTree, err error) {
	gt = sgf.NewGameTree()

	// a GameTree is a sequence of nodes ...
	var sequence []*sgf.Node
	var lastItem item
	sequence, lastItem, err = parseSequence(l)
	if err != nil {
		return
	}
	gt.Sequence = sequence

	// ... followed by 0 or more GameTrees
	for {
		// the first time through this loop we are looking at the lastItem returned
		// by the parseSequece function: that is the item which followed the
		// sequence
		switch lastItem.typ {
		case itemCloseParen:
			// consume the closing ) of the GameTree and return
			return
		case itemOpenParen:
			// we have a subtree
			var subtree *sgf.GameTree
			subtree, err = parseGameTree(l)
			if err != nil {
				return
			}
			gt.AddSubtree(subtree)

			// the subtree has consumed its closing ) so lets go look for our own
			// closing ) or the ( of another subtree
			var ok bool
			lastItem, ok = <-l.items
			if !ok {
				err = fmt.Errorf("parse: lexer channel closed unexpectedly")
				return
			}
		case itemError:
			err = fmt.Errorf("%s", lastItem.val)
			return
		default:
			// only ( and ), that is the opening ( of a new subtree or the ) of this
			// GameTree, are legal after the sequence
			err = fmt.Errorf("parse: unexpected item: %s", lastItem)
			return
		}
	}
}

func parseSequence(l *lexer) (sequence []*sgf.Node, lastItem item, err error) {
	sequence = []*sgf.Node{}

	var ok bool
	lastItem, ok = <-l.items
	if !ok {
		err = fmt.Errorf("parse: lexer channel closed unexpectedly")
		return
	}

	for {
		switch lastItem.typ {
		case itemSemiColon:
			// a ; means that we are at the beginning of a node
			var n *sgf.Node
			n, lastItem, err = parseNode(l)
			if err != nil {
				return
			}
			sequence = append(sequence, n)
			// get the node and then go around again taking a look at the item which
			// followed the node
		case itemError:
			err = fmt.Errorf("%s", lastItem.val)
			return
		default:
			// if the item following the node is not a ; , we are apparently
			// done with the sequence
			if len(sequence) == 0 {
				// a sequence must have at least one node in it
				err = fmt.Errorf("parse: the GameTree sequence must have at least one node in it")
				return
			}
			return
		}
	}
}

func parseNode(l *lexer) (n *sgf.Node, lastItem item, err error) {
	n = sgf.NewNode()

	var currentProperty *sgf.Property
	var currentPropertyValue *sgf.PropertyValue

	for lastItem = range l.items {
		switch lastItem.typ {
		case itemSemiColon, itemCloseParen, itemOpenParen:
			// if it is the beginning of another node, the end of this GameTree, or
			// the beginning of a subtree
			if currentProperty != nil {
				// store the last property in the node; it is entirely possible to have
				// an empty node, that is a node with an empty Properties slice
				err = n.AddProperty(currentProperty)
				if err != nil {
					return
				}
			}
			return
		case itemPropertyIdent:
			if currentProperty != nil {
				err = n.AddProperty(currentProperty)
				if err != nil {
					return
				}
			}
			currentProperty, err = sgf.NewProperty(string(lastItem.val), nil)
			if err != nil {
				return
			}
		case itemOpenBracket:
			currentPropertyValue = sgf.NewEmptyPropertyValue()
		case itemPropertyValue:
			if currentPropertyValue != nil {
				err = currentPropertyValue.SetValue(string(lastItem.val))
				if err != nil {
					return
				}
			} else {
				err = fmt.Errorf("parse: got a property value without an opening [")
				return
			}
		case itemCloseBracket:
			if currentPropertyValue != nil {
				err = currentProperty.AddValue(currentPropertyValue)
				if err != nil {
					return
				}
				currentPropertyValue = nil
			} else {
				err = fmt.Errorf("parse: got a closing ] without an opening [")
				return
			}
		case itemError:
			err = fmt.Errorf("%s", lastItem.val)
			return
		default:
			err = fmt.Errorf("parse: unexpected item: %s", lastItem)
			return
		}
	}
	err = fmt.Errorf("parse: lexer channel closed unexpectedly")
	return
}
