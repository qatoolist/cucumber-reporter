Feature: Step statuses

  Scenario: Passing
    Given ok

  Scenario: Pending
    Given ok
    And this is pending
    And ok

  Scenario: Undefined
    Given ok
    And nobody defined this step
    And ok

  Scenario: Ambiguous
    Given ok
    And this matches twice
    And ok

  Scenario: Failing then skipped
    Given it fails
    And ok
