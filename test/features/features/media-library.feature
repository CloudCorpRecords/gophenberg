Feature: Managing the media library
  The library is where an administrator finds, describes and removes
  uploads. Descriptions feed the editor and the public site, so edits
  must be safe under concurrent use.

  Background:
    Given a running Gophenberg holding a seeded media library
    And a signed in administrator

  Scenario: The library lists newest uploads first
    When the administrator uploads a new image named "jetty.jpg"
    Then the library lists "jetty" first

  @wip
  Scenario Outline: Sorting the library by title
    When the administrator lists the library sorted by "title" "<order>"
    Then the library lists <titles> in that order

    Examples:
      | order | titles                         |
      | asc   | "cliff", "harbor" and "manual" |
      | desc  | "manual", "harbor" and "cliff" |

  @wip
  Scenario: A sort naming no order lists in descending order
    When the administrator lists the library sorted by "title"
    Then the library lists "manual", "harbor" and "cliff" in that order

  @wip
  Scenario Outline: Sorting the library by a date
    Given "cliff" was added on "2026-03-10" and last modified on "2026-09-10"
    And "manual" was added and last modified on "2026-05-10"
    And "harbor" was added and last modified on "2026-07-10"
    When the administrator lists the library sorted by "<sort>" "<order>"
    Then the library lists <titles> in that order

    Examples:
      | sort     | order | titles                         |
      | date     | asc   | "cliff", "manual" and "harbor" |
      | date     | desc  | "harbor", "manual" and "cliff" |
      | modified | asc   | "manual", "harbor" and "cliff" |
      | modified | desc  | "cliff", "harbor" and "manual" |

  @wip
  Scenario Outline: Items modified at the same moment list the newest upload first
    Given "harbor" and "manual" were modified at the same moment
    And "cliff" was modified after them
    When the administrator lists the library sorted by "modified" "<order>"
    Then the library lists <titles> in that order

    Examples:
      | order | titles                         |
      | asc   | "manual", "harbor" and "cliff" |
      | desc  | "cliff", "manual" and "harbor" |

  @wip
  Scenario Outline: Sorting the library by the name of the author
    Given "cliff" was uploaded by an author whose name sorts first
    When the administrator lists the library sorted by "author" "<order>"
    Then the library lists <titles> in that order

    Examples:
      | order | titles                         |
      | asc   | "cliff", "manual" and "harbor" |
      | desc  | "manual", "harbor" and "cliff" |

  Scenario: Each listed item names its author
    Given the image "pier.jpg" was uploaded by another author
    When the administrator lists the library
    Then every listed item names the account that uploaded it

  Scenario: Searching the library by name
    When the administrator searches the library for "harbor"
    Then the library lists only media matching "harbor"

  @wip
  Scenario Outline: Searching the library by <field>
    Given the image "cliff" carries the <field> "A lighthouse above the bay"
    When the administrator searches the library for "lighthouse"
    Then the library lists only "cliff"

    Examples:
      | field       |
      | caption     |
      | description |

  Scenario: Filtering the library to images
    When the administrator filters the library to images
    Then the library lists no plain files
    And the library still lists every image

  Scenario: Filtering the library to the kinds a block accepts
    When the administrator filters the library to images and video
    Then the library lists no plain files
    And the library still lists every image

  @wip
  Scenario: Filtering the library to the uploads of one author
    Given the image "pier.jpg" was uploaded by another author
    When the administrator filters the library to the uploads of that author
    Then the library lists only "pier"

  @wip
  Scenario: Leaving the uploads of one author out of the library
    Given the image "pier.jpg" was uploaded by another author
    When the administrator leaves the uploads of that author out of the library
    Then the library lists only "harbor", "cliff" and "manual"

  @wip
  Scenario Outline: Filtering the library by the date added
    Given "cliff" was added on "2026-03-10" and last modified on "2026-09-10"
    And "manual" was added and last modified on "2026-05-10"
    And "harbor" was added and last modified on "2026-07-10"
    When the administrator filters the library to the items added <range>
    Then the library lists only <titles>

    Examples:
      | range                                      | titles               |
      | before "2026-06-01"                        | "cliff" and "manual" |
      | after "2026-06-01"                         | "harbor"             |
      | after "2026-04-01" and before "2026-06-01" | "manual"             |
      | before "2026-05-10"                        | "cliff"              |
      | after "2026-05-10"                         | "harbor"             |

  @wip
  Scenario Outline: Filtering the library by the date modified
    Given "cliff" was added on "2026-03-10" and last modified on "2026-09-10"
    And "manual" was added and last modified on "2026-05-10"
    And "harbor" was added and last modified on "2026-07-10"
    When the administrator filters the library to the items modified <range>
    Then the library lists only <titles>

    Examples:
      | range                                      | titles               |
      | before "2026-06-01"                        | "manual"             |
      | after "2026-06-01"                         | "harbor" and "cliff" |
      | after "2026-06-01" and before "2026-08-01" | "harbor"             |
      | before "2026-07-10"                        | "manual"             |
      | after "2026-07-10"                         | "cliff"              |

  Scenario: Paging through the library
    When the administrator opens the second page of two per page
    Then the library reports the total while listing the remainder

  Scenario: A page larger than the cap is cut to the cap
    Given the admin lists open at 10 items and carry at most 50 a page
    When the administrator lists the library 500 at a time
    Then the library was paged 50 at a time

  Scenario: A listing naming no page size opens at the configured one
    Given the admin lists open at 10 items and carry at most 50 a page
    When the administrator lists the library
    Then the library was paged 10 at a time

  Scenario: Describing an image
    When the administrator describes the image "harbor"
    Then reading "harbor" back returns every saved description

  Scenario: An edit that changes nothing is not a conflict
    When the administrator saves the image "harbor" unchanged
    Then the request is accepted

  Scenario: A stale description does not overwrite a newer one
    Given two administrators read the image "harbor"
    When both save a description in turn
    Then the second save is refused as a conflict
    And the first description is untouched

  Scenario: Deleting an image removes every trace
    When the administrator permanently deletes the image "harbor"
    Then the library does not list "harbor"
    And the files of "harbor" are gone from the media directory

  Scenario: Deleting an image twice reports it missing
    When the administrator permanently deletes the image "harbor"
    And the administrator deletes "harbor" again
    Then the request reports the media does not exist

  Scenario: Reading media that was never stored reports it missing
    When the administrator asks for media that was never stored
    Then the request reports the media does not exist

  Scenario Outline: A listing the library cannot make sense of is refused
    When the administrator lists the library with <parameters>
    Then the request is refused as a bad listing

    Examples:
      | parameters       |
      | the type "audio" |
      | the page "0"     |

  Scenario Outline: A page size the library cannot read is refused
    When the administrator lists the library with <parameters>
    Then the request is refused with the code "list_parameters_invalid"

    Examples:
      | parameters        |
      | the page size "0" |
      | the page size "x" |

  @wip
  Scenario Outline: A sort the library does not offer is refused
    When the administrator lists the library with <parameters>
    Then the request is refused with the code "list_parameters_invalid"

    Examples:
      | parameters                |
      | the sort "slug"           |
      | the sort order "sideways" |

  @wip
  Scenario Outline: A filter value the library cannot read is refused
    When the administrator lists the library with <parameters>
    Then the request is refused with the code "list_parameters_invalid"

    Examples:
      | parameters                           |
      | the author "nobody"                  |
      | the excluded author "nobody"         |
      | the date added before "yesterday"    |
      | the date added after "yesterday"     |
      | the date modified before "yesterday" |
      | the date modified after "yesterday"  |
