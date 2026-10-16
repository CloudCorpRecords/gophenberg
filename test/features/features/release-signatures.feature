Feature: Release files carry signatures anyone can check
  A release file is signed with Ed25519 over its SHA-384 digest, and the
  signature is written as base64url without padding, the form FAIR
  packages are signed with today. Anyone who holds the public key can
  check that a file is the one the release key signed.

  Scenario: A file signed with a fresh key verifies
    Given a fresh release key
    And a release file holding "gophenberg release archive"
    When the release key signs the file
    Then the signature holds for the file under the release key

  Scenario: A changed byte fails the check
    Given a fresh release key
    And a release file holding "gophenberg release archive"
    And the release key signs the file
    When one byte of the file changes
    Then the check refuses the file

  Scenario: A live FAIR signature verifies
    Given the live FAIR signer "Query Monitor 3.20.2"
    When its signature is checked against its digest under its key
    Then the check holds

  Scenario: A signature in standard base64 is refused
    Given the live FAIR signer "Query Monitor 3.20.2"
    When its signature is rewritten in standard base64 with padding
    And its signature is checked against its digest under its key
    Then the check refuses the signature, naming base64url without padding
