---
title: Content types
description: Registering kinds of content and choosing the addresses they answer under.
---

A content type is one kind of content your site holds. Every site
starts with Posts. This page shows how to add more kinds, like Pages
or Guides, and how each kind gets its own addresses.

Open **Content Types** in the admin menu. The table lists every
registered type, the address it answers under, and what may be done
to it.

## Registering a type

Press **Add New Type** and give it a singular and a plural name, like
Guide and Guides. Gophenberg derives the rest: the plural becomes the
route word, so Guides answer under `/guides`.

The **Description** is optional. Write one sentence about what the
type holds, like "Every guide this site keeps." Gophenberg stores it
exactly as you typed it. Posts start with "Manage the posts on this
site."

The names and descriptions Gophenberg gives its own types, like Posts,
show in the language you read the admin in while they still hold the
English Gophenberg gave them. Once someone changes one, it shows as
typed, like anything else you write here. A type a plugin declared
always shows the words its plugin gives it.

The description shows under the title of the type's list. A type
with no description shows a sentence of the admin's own there
instead, "Manage the items of this content type.", in the language
you read the admin in.

The new type appears in the admin menu right away, with the same list
screen and editor Posts have. Press **Fields** on a type to give its
items typed fields of their own, covered in
[fields](/guides/fields/).

## Describing a type

**Describe** opens the description of a type so you can rewrite it.
Empty the field and save to clear it. A type a plugin declared has no
**Describe** button, because it keeps the description its plugin
gives it.

The description travels with its type in **Export definitions**. A
file exported by an older release holds no descriptions, and
importing it leaves each description as it stands. See
[moving definitions between sites](/guides/fields/#moving-definitions-between-sites).

## Addresses and the route word

The route word is the first part of every address a type serves. A
guide with the slug `first-steps` answers at `/guides/first-steps`.

One type owns the root of the site instead of a route word. It is
marked **Default** in the table, and its items answer at `/{slug}`
directly. Press **Make default** on another type to hand the root
over.

The type that gives up the root takes a route word made from its
plural label, so **Posts** moves to `/posts`. When the label makes no
usable word, such as `3D Models` or a label in another alphabet, the
type uses its key instead. A type keyed `page` cannot fall back this
way, because `/page/2` is the second page of a listing, so give it a
label that makes a word first. When an import brings a new default
type and your site keeps its own root, the new type gets its route
word the same way.

**Change address** moves a type to a new route word. Every address of
that type moves with it, at once, including nested items. Links kept
outside your site to the old addresses stop working, which is why the
dialog asks you to confirm.

## Nesting

A type marked **Nests** can hold items inside items. The editor's
Document panel then offers a **Parent** select, and a child's address
carries the chain of its parents: `/pages/about/team` is the page
`team` inside `about`.

Nesting goes at most ten levels deep. An item holding children cannot
be trashed until its children move or go first.

**Let items nest** turns nesting on for a type, at any time. **Stop
nesting** turns it off, and is refused while any item of the type
still sits inside another, the ones in the trash included. Move those
to the top level first. An imported definitions file meets the same
rule: a type it marks flat keeps nesting while its items nest, and the
plan says so before you apply it. A plugin that marks its own type
flat meets it too, and the type keeps nesting until those items move.

## Turning a type off

**Deactivate** hides a type without deleting anything. Its menu entry,
its archive, and its items stop being served, and everything comes
back with **Activate**.

**Delete** removes a type for good, and is refused while the type
still holds content. It asks first, in a dialog naming the type, and
Cancel has the focus when the dialog opens. When the server refuses,
the dialog stays open and says why. The default type can be neither
deactivated nor deleted, so the root always answers.
