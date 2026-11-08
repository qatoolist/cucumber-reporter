Feature: Retry final non-pass

  Scenario: Fails then pending
    Given attempt step a

  Scenario: Fails then skipped
    Given attempt step b

  Scenario: Fails then passes
    Given attempt step c
