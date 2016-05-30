//
// Copyright (c) 2016 Konstanin Ivanov <kostyarin.ivanov@gmail.com>.
// All rights reserved. This program is free software. It comes without
// any warranty, to the extent permitted by applicable law. You can
// redistribute it and/or modify it under the terms of the Do What
// The Fuck You Want To Public License, Version 2, as published by
// Sam Hocevar. See LICENSE.md file for more details or see below.
//

//
//        DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE
//                    Version 2, December 2004
//
// Copyright (C) 2004 Sam Hocevar <sam@hocevar.net>
//
// Everyone is permitted to copy and distribute verbatim or modified
// copies of this license document, and changing it is allowed as long
// as the name is changed.
//
//            DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE
//   TERMS AND CONDITIONS FOR COPYING, DISTRIBUTION AND MODIFICATION
//
//  0. You just DO WHAT THE FUCK YOU WANT TO.
//

//
package grokky

// http://play.golang.org/p/vb18r_OZkK

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var patternRegexp = regexp.MustCompile(`\%\{(\w+)(\:(\w+))?}`)

var (
	// Empty pattern name
	ErrEmptyName = errors.New("an empty name")
	// Empty expression
	ErrEmptyExpression = errors.New("an empty expression")
	// Pattern with given name already exists
	ErrAlreadyExist = errors.New("the pattern already exist")
	// Patter with given name does not exists
	ErrNotExist = errors.New("pattern doesn't exist")
)

// helpers

func split(s string) (name, sem string) {
	ss := patternRegexp.FindStringSubmatch(s)
	if len(ss) >= 2 {
		name = ss[1]
	}
	if len(ss) >= 4 {
		sem = ss[3]
	}
	return
}

func wrap(s string) string { return "(" + s + ")" }

// host

// Host is a patterns collection. Feel free to
// delete the Host after all patterns (that you need)
// are created. Think of it as a kind of factory.
type Host map[string]string

// New retuns new empty host
func New() Host { return make(Host) }

// Add a new pattern to the Host. If pattern with given name
// already exists the ErrAlreadyExists will be retuned.
func (h Host) Add(name, expr string) error {
	if name == "" {
		return ErrEmptyName
	}
	if expr == "" {
		return ErrEmptyExpression
	}
	if _, ok := h[name]; ok {
		return ErrAlreadyExist
	}
	if _, err := h.compileExternal(expr); err != nil {
		return err
	}
	h[name] = expr
	return nil
}

func (h Host) compile(name string) (*Pattern, error) {
	expr, ok := h[name]
	if !ok {
		return nil, ErrNotExist
	}
	return h.compileExternal(expr)
}

func (h Host) compileExternal(expr string) (*Pattern, error) {
	// find subpatterns
	subs := patternRegexp.FindAllString(expr, -1)
	// this semantics set
	ts := make(map[string]struct{})
	// chek: does subpatterns exist into this Host?
	for _, s := range subs {
		name, sem := split(s)
		if _, ok := h[name]; !ok {
			return nil, fmt.Errorf("the '%s' pattern doesn't exist", name)
		}
		ts[sem] = struct{}{}
	}
	// if there are not subpatterns
	if len(subs) == 0 {
		r, err := regexp.Compile(expr)
		if err != nil {
			return nil, err
		}
		p := new(Pattern)
		p.Regexp = *r
		return p, nil
	}
	// split
	spl := patternRegexp.Split(expr, -1)
	// concat it back
	msi := make(map[string]int)
	order := 1 // semantic order
	var res string
	for i := 0; i < len(spl)-1; i++ {
		// split part
		splPart := spl[i]
		order += capCount(splPart)
		// subs part
		sub := subs[i]
		subName, subSem := split(sub)
		p, err := h.compile(subName)
		if err != nil {
			return nil, err
		}
		sub = p.String()
		subNumSubexp := p.NumSubexp()
		subNumSubexp++
		sub = wrap(sub)
		if subSem != "" {
			msi[subSem] = order
		}
		res += splPart + sub
		// add sub semantics to this semantics
		for k, v := range p.s {
			if _, ok := ts[k]; !ok {
				msi[k] = order + v
			}
		}
		// increse the order
		order += subNumSubexp
	} // last spl
	res += spl[len(spl)-1]
	r, err := regexp.Compile(res)
	if err != nil {
		return nil, err
	}
	p := new(Pattern)
	p.Regexp = *r
	p.s = msi
	return p, nil
}

// Get pattern by name from the Host
func (h Host) Get(name string) (*Pattern, error) {
	return h.compile(name)
}

// Compile and get pattern without name (and without adding it to this Host)
func (h Host) Compile(expr string) (*Pattern, error) {
	if expr == "" {
		return nil, ErrEmptyExpression
	}
	return h.compileExternal(expr)
}

// Pattern is pattern.
// Feel free to use the Pattern as regexp.Regexp.
// The Find method has different behavior.
type Pattern struct {
	regexp.Regexp
	s map[string]int
}

// Find returns map (name->match) on input. The map can be empty.
func (p *Pattern) Find(input string) map[string]string {
	ss := p.FindStringSubmatch(input)
	r := make(map[string]string)
	if len(ss) <= 1 {
		return r
	}
	for sem, order := range p.s {
		r[sem] = ss[order]
	}
	return r
}

// Names retuns all names that this pattern has
func (p *Pattern) Names() (ss []string) {
	ss = make([]string, 0, len(p.s))
	for k := range p.s {
		ss = append(ss, k)
	}
	return
}

var lineRegexp = regexp.MustCompile(`^(\w+)\s+(.+)$`)

func (h Host) addFromLine(line string) error {
	sub := lineRegexp.FindStringSubmatch(line)
	if len(sub) == 0 { // not match
		return nil
	}
	return h.Add(sub[1], sub[2])
}

// AddFromFile appends all patterns from the file to this Host.
func (h Host) AddFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if err := h.addFromLine(scanner.Text()); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

// http://play.golang.org/p/1rPuziYhRL

var (
	nonCapLeftRxp  = regexp.MustCompile(`\(\?[imsU\-]*\:`)
	nonCapFlagsRxp = regexp.MustCompile(`\(?[imsU\-]+\)`)
)

// cap count
func capCount(in string) int {
	leftParens := strings.Count(in, "(")
	nonCapLeft := len(nonCapLeftRxp.FindAllString(in, -1))
	nonCapBoth := len(nonCapFlagsRxp.FindAllString(in, -1))
	escapedLeftParens := strings.Count(in, `\(`)
	return leftParens - nonCapLeft - nonCapBoth - escapedLeftParens
}