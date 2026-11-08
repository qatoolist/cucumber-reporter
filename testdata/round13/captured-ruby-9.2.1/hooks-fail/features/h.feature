Feature: Hook failures and output

  @before_fail
  Scenario: Before fails
    Given a step

  @after_fail
  Scenario: After fails after pass
    Given a step

  @afterstep_fail
  Scenario: AfterStep fails
    Given a step
    And a step

  @output
  Scenario: Hooks attach everything and pass
    Given a step

  @output @after_fail
  Scenario: Hooks attach then After fails
    Given a step
