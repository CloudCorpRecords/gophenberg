---
title: The media library
description: Uploading pictures and files, describing them, and placing them in a post.
---

The Media screen holds every file you have uploaded. Pictures,
video, audio and documents all live there, and the editor takes
what it places from the same library.

## Uploading a file

Open Media in the menu and press **Add Media File** at the top of the
screen. You can pick several files at once, or drop them straight
onto the library, which reads "Drop files to upload" while you drag.
Either way the upload starts at once, every file sent side by side,
and the files appear in the grid when the server has stored them.

A short message at the bottom of the screen names the file you
uploaded, or counts the files when you sent several. A file the
server refuses is not counted. Its reason shows in a notice above
the library, and when you sent several files each refused one is
named on its own line.

Gophenberg accepts JPEG, PNG, GIF and WebP pictures, MP4 and WebM
video, MP3, M4A, WAV, OGG and FLAC audio, PDF documents and ZIP
archives. Anything else is refused, and the answer says why.

A file is checked before it is stored, so refusing one never leaves
anything behind. An upload is turned away when its type is not on
that list, when its contents do not match its name, when it is
larger than the upload limit, or when it is a picture the server
cannot read.

## What happens to a picture

Every picture you upload is stored twice over: the file exactly as
you sent it, and a set of smaller copies the editor can choose
from. The copies are named Thumbnail, Medium, Large and Full Size,
which is what the editor's Resolution setting lists.

Copies saved as JPEG are saved at a quality of 82 out of 100,
unless an administrator chose another number under **Settings**.
A lower number makes lighter files that look rougher. The change
applies to pictures uploaded after it, so copies already on disk
stay as they were.

Two things are corrected on the way in. A photo taken sideways is
turned upright, so it is stored the way you saw it on the camera.
A very large photo also gains a display copy bounded to 2560
pixels, and that copy is what a post shows. Your original stays on
disk either way.

An animated GIF is the exception. It is stored exactly as
uploaded and gains no smaller copies, because making them would
throw the animation away.

## Finding and describing

The library opens as a grid of thumbnails. The **Layout** control
switches it to a table, which adds the file name, type and date.
Search matches titles and file names, and the filter narrows the
list to images or to plain files. When nothing matches, the library
says `No media items found.` An empty library says `No media items
yet.` and how to add one.

Every item carries four descriptions, all editable through
**Describe**:

- **Title** names the item in the library.
- **Alt text** describes a picture to someone who cannot see it.
  Leave it empty when the picture is decorative.
- **Caption** is the line shown under a picture in a post.
- **Description** is a longer note for your own use.

If someone else changes an item while you are describing it, your
save is refused rather than quietly overwriting theirs. Reload and
make the change again.

## Placing media in a post

In the editor, any media block offers two ways to fill itself.
**Upload** takes a file from your computer and adds it to the
library on the way in. **Media Library** opens the same library
you see on the Media screen, so you can place a file you already
uploaded without uploading it twice.

The inserter also has a Media tab listing your images, video and
audio, which places an item with a single click.

A picture placed in a post keeps pointing at the library. Deleting
it from the library does not repair the post, so check where a
file is used before removing it.

A **Media** or **Gallery** [field](/guides/fields/) opens the same
library from the Document panel, and the same warning applies.

## Deleting

**Delete Permanently** removes an item and every copy of it from
disk. There is no media trash, so the confirmation is the only
step between you and a deleted file. You can select several items
and delete them together.
