Feature: Operators update the core from the command line
  A server install keeps each build of Gophenberg in its own folder under
  an install root, with a current link naming the build that runs and a
  previous link naming the build before it. update:apply checks the
  signed index again, downloads the archive, checks its checksum and its
  signature, keeps a backup of the database and points current at the new
  build, then asks for a restart. At its first start the new build counts
  the attempt and checks itself. A healthy start keeps the update. A build
  that fails goes back to the previous one, and that version is never
  picked again on its own unless a person retries it. An update that
  raised the minimum version never goes back on its own, since the older
  build refuses the database the newer one migrated, so the site stays
  down and shows how to restore the backup. An install with no install
  root, like the Docker image, and a custom build keep the manual steps.

  Background:
    Given the settings point at an empty database
    And the operator ran "migrate"

  Scenario: Applying an update points current at the new build and keeps the old one
    Given an install root running this version as an official build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When the operator runs "update:apply"
    Then the command exits with code 75
    And the command asks for a restart to start "99.0.0"
    And the current build is "99.0.0"
    And the previous build is this version

  Scenario: Applying an update keeps a backup of the database first
    Given an install root running this version as an official build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When the operator runs "update:apply"
    Then the command exits with code 75
    And the install root keeps a database backup named after this version

  Scenario Outline: Applying an update refuses <case>
    Given an install root running this version as an official build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    And <change>
    When the operator runs "update:apply"
    Then the command exits with code 1
    And the error says "<error>"
    And the current build is still this version
    And the install root holds no download and no build of "99.0.0"

    Examples:
      | case                              | change                                    | error                                                                       |
      | an index no listed key signed     | only a stranger signed the index          | refused the index, no release or standby key the list names signed it       |
      | an archive whose checksum differs | the archive changed after it was released | the one the release record names                                            |
      | an archive no listed key signed   | only a stranger signed the archive        | refused the archive, no release or standby key the key list names signed it |

  Scenario Outline: A version that failed is never picked again on its own
    Given an install root running this version as an official build
    And version "99.0.0" failed its start check after an update and went back
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When the operator runs "<line>"
    Then the command exits with code 1
    And the error says "Gophenberg 99.0.0 failed"
    And the error says "update:apply -to 99.0.0 -retry"
    And the current build is still this version

    Examples:
      | line                    |
      | update:apply            |
      | update:apply -to 99.0.0 |

  Scenario: A person retries a version that failed
    Given an install root running this version as an official build
    And version "99.0.0" failed its start check after an update and went back
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When the operator runs "update:apply -to 99.0.0 -retry"
    Then the command exits with code 75
    And the current build is "99.0.0"
    And the update status lists no refused version

  Scenario: An install with no install root, like the Docker image, keeps the manual steps
    Given an official build with no install root, like the Docker image
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When the operator runs "update:apply"
    Then the command exits with code 1
    And the error says "GOPHENBERG_INSTALL_ROOT is empty"
    And the error says "update by hand to Gophenberg 99.0.0"

  Scenario: A custom build keeps the manual steps
    Given an install root running this version as a custom build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When the operator runs "update:apply"
    Then the command exits with code 1
    And the error says "custom build"
    And the error says "rebuild it on Gophenberg 99.0.0"
    And the current build is still this version

  Scenario: A healthy first start keeps the update
    Given an install root where an update switched from "0.18.0" to this version
    When the site starts
    Then the update status says the update to this version is kept
    And the current build is still this version
    And the previous build is still "0.18.0"

  Scenario: A build that keeps failing to start goes back to the previous one
    Given an install root where an update switched from "0.18.0" to this version
    And this version already tried to start 3 times
    When the site starts
    Then the command exits with code 76
    And the current build is "0.18.0"
    And the update status lists this version as refused

  Scenario: A failed update that raised the minimum version stays down and shows the restore steps
    Given an install root where an update switched from "0.18.0" to this version
    And the update raised the minimum version, with a backup taken before the switch
    And this version already tried to start 3 times
    When the site starts
    Then the command exits with code 76
    And the current build is still this version
    And the error names the backup taken before the switch
    And the error says "restore the backup taken before the update"
    And the update status names the backup taken before the switch

  Scenario Outline: The service manager takes the site back after <failure>
    Given an install root where an update switched from this version to "99.0.0"
    When the service manager runs "update:rollback -after-failure" with <variable> set to "<value>"
    Then the command succeeds
    And the current build is this version
    And the update status lists version "99.0.0" as refused

    Examples:
      | failure                           | variable               | value           |
      | the new build failed its check    | MONITOR_EXIT_STATUS    | 76              |
      | the new build hit the start limit | MONITOR_SERVICE_RESULT | start-limit-hit |

  Scenario: The service manager leaves an update that passed its check alone
    Given an install root where an update switched from this version to "99.0.0"
    And the new build passed its check
    When the service manager runs "update:rollback -after-failure" with MONITOR_SERVICE_RESULT set to "start-limit-hit"
    Then the command exits with code 1
    And the current build is still "99.0.0"

  Scenario Outline: The service manager does not take back an update that raised the minimum version after <failure>
    Given an install root where an update switched from this version to "99.0.0"
    And the update raised the minimum version, with a backup taken before the switch
    And the new build already tried to start 3 times
    When the service manager runs "update:rollback -after-failure" with <variable> set to "<value>"
    Then the command exits with code 1
    And the current build is still "99.0.0"
    And the error names the backup taken before the switch
    And the error says "restore the backup taken before the update"
    And the update status names the backup taken before the switch

    Examples:
      | failure                           | variable               | value           |
      | the new build failed its check    | MONITOR_EXIT_STATUS    | 76              |
      | the new build hit the start limit | MONITOR_SERVICE_RESULT | start-limit-hit |
