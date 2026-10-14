Feature: Plugins read account names
  A plugin that shows who wrote something keeps account ids and asks the
  site for the names behind them. It asks for many accounts in one call,
  so a page of fifteen authors costs one read, and it never sees an
  email address.

  Background:
    Given these accounts
      | name               | email                   | role   | state       |
      | Maria Perez        | maria@example.com       | member | confirmed   |
      | Second Author      | second@example.com      | author | confirmed   |
      | Disabled Member    | disabled@example.com    | member | disabled    |
      | Unconfirmed Member | unconfirmed@example.com | member | unconfirmed |
    And a running Gophenberg serving the plugin "names", which shows the accounts it asks for

  Scenario: A plugin reads many account names in one call
    When the plugin "names" asks for "Maria Perez" and "Second Author"
    Then it shows "Maria Perez" with the role "member"
    And it shows "Second Author" with the role "author"
    And the site read the accounts once

  Scenario: An id the site does not hold is left out of the answer
    When the plugin "names" asks for "Maria Perez" and an account the site does not hold
    Then it shows only "Maria Perez"

  Scenario: An id asked twice comes back once
    When the plugin "names" asks for "Maria Perez" and "Maria Perez"
    Then it shows only "Maria Perez"
    And the site read the accounts once

  Scenario: Asking for no accounts reads nothing
    When the plugin "names" asks for no accounts
    Then it shows nobody
    And the site read no accounts

  Scenario: A disabled account keeps its name and is marked disabled
    When the plugin "names" asks for "Disabled Member"
    Then it shows "Disabled Member" with the role "member"
    And it shows "Disabled Member" as disabled

  Scenario: An account that never confirmed its address is left out
    When the plugin "names" asks for "Maria Perez" and "Unconfirmed Member"
    Then it shows only "Maria Perez"

  Scenario: The answer never holds an email address
    Given the site draws avatars from "gravatar"
    When the plugin "names" asks for "Maria Perez" and "Second Author"
    Then the answer holds no email address

  Scenario: With initials the site draws the avatar and sends nothing out
    Given the site draws avatars from "initials"
    When the plugin "names" asks for "Maria Perez" at 48 pixels
    Then the avatar of "Maria Perez" is a picture the site drew with the letter "M"

  Scenario: With Gravatar the avatar carries a hash of the address at the asked size
    Given the site draws avatars from "gravatar"
    When the plugin "names" asks for "Maria Perez" at 48 pixels
    Then the avatar of "Maria Perez" is the Gravatar of "maria@example.com" at 48 pixels

  Scenario: With avatars off the avatar is empty
    Given the site draws avatars from "off"
    When the plugin "names" asks for "Maria Perez"
    Then the avatar of "Maria Perez" is empty
