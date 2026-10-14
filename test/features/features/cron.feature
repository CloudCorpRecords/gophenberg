Feature: Cron jobs
  A site on shared hosting has no process that stays awake, so work that
  must happen on a rhythm falls due instead. The first visit after its
  time runs it once the page is answered, so the visitor never waits.
  The host's cron can call the site too, holding a secret, so a quiet site
  still runs its cron jobs.

  Background:
    Given a running Gophenberg with the cron job "tidy" every hour and the cron secret set

  Scenario: The first visit runs a job that has never run
    When a visitor opens the home page
    Then the job "tidy" has run 1 time

  Scenario: A visitor gets the page while a slow job is still running
    Given the job "tidy" waits until it is let go
    When a visitor opens the home page
    Then the visitor got the page
    And the job "tidy" is still running
    And the job "tidy" finishes once it is let go

  Scenario: A visit inside the visit pace checks nothing
    Given a visitor opened the home page
    When 30 seconds pass
    And a visitor opens the home page
    Then the site checked for cron jobs 1 time

  Scenario: A job runs again once its hour has passed, and not before
    Given a visitor opened the home page
    When 59 minutes pass
    And a visitor opens the home page
    Then the job "tidy" has run 1 time
    When 1 minute passes
    And a visitor opens the home page
    Then the job "tidy" has run 2 times

  Scenario: Two visits at the same moment run a cron job once
    When two visitors open the home page at the same moment
    Then the job "tidy" has run 1 time

  Scenario: A job three days late runs once and keeps its hour
    Given a visitor opened the home page at "2026-10-13T10:00:00Z"
    When the clock reads "2026-10-16T10:15:00Z"
    And a visitor opens the home page
    Then the job "tidy" has run 2 times
    And the job "tidy" falls due next at "2026-10-16T11:00:00Z"

  Scenario: A failing job is recorded and waits for its next due time
    Given the job "tidy" fails with "the disk is full"
    And a visitor opened the home page at "2026-10-13T10:00:00Z"
    Then the job "tidy" last failed with "the disk is full"
    And the job "tidy" falls due next at "2026-10-13T11:00:00Z"

  Scenario: A job that runs past the time limit is cut short and recorded
    Given the job "tidy" runs until it is told to stop
    When a visitor opens the home page
    Then the job "tidy" is cut short at its time limit
    And the job "tidy" last failed with "context deadline exceeded"

  Scenario: A job that panics is recorded and the next job still runs
    Given the cron job "archive" every hour that panics
    When a visitor opens the home page
    Then the job "archive" last failed with "panic"
    And the job "tidy" has run 1 time

  Scenario: A job another server holds is left alone until its lease runs out
    Given another server holds the job "tidy"
    When a visitor opens the home page
    Then the job "tidy" has run 0 times
    When 1 minute passes
    And a visitor opens the home page
    Then the job "tidy" has run 1 time

  Scenario: The cron address runs the cron jobs and counts them
    When the host's cron calls the cron address with the secret
    Then the call answers that 1 job ran
    And the job "tidy" has run 1 time

  Scenario: The cron address answers a wrong secret like a missing page
    Given the site runs with visits turned off
    When the host's cron calls the cron address with a wrong secret
    Then the call is answered like a missing page
    And the job "tidy" has run 0 times

  Scenario: The cron address answers a call with no secret like a missing page
    Given the site runs with visits turned off
    When the host's cron calls the cron address with no secret
    Then the call is answered like a missing page
    And the job "tidy" has run 0 times

  Scenario: Without a cron secret the address does not exist
    Given the site runs without a cron secret
    When the host's cron calls the cron address with the secret
    Then the call is answered like a missing page

  Scenario: With visits turned off, a visit runs nothing and the cron address still runs the jobs
    Given the site runs with visits turned off
    When a visitor opens the home page
    Then the job "tidy" has run 0 times
    When the host's cron calls the cron address with the secret
    Then the call answers that 1 job ran

  Scenario: A cron call is never cached
    When the host's cron calls the cron address with the secret
    Then the answer is kept out of every cache
