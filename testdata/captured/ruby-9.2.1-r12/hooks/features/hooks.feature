Feature: Hook failures

  Scenario: Plain pass
    Given a step passes

  @before_fails
  Scenario: Before hook fails
    Given a step passes

  @after_fails
  Scenario: After hook fails
    Given a step passes

  @afterstep_fails
  Scenario: AfterStep hook fails
    Given a step passes
    And another step passes
