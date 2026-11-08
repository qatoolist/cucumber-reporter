@search
Feature: Search
  Search **must** be fast.

  Scenario: Same name in two features
    When I add 1 items
    Then the cart has 1 items

  Scenario: Nested steps pass
    When I do the composite action

  Scenario: Nested step fails inside
    When I do the failing composite action

  Scenario: Deep multiline exception with backtrace
    Then the deep call fails with "search index exploded"

  Scenario: Pending with a message
    Given the store "S" is open
    Then the feature is pending
    And the cart has 0 items

  Scenario: Undefined step with snippet
    Given the store "S" is open
    When I search for "ünï 🎉" with 3 filters
    Then the cart has 0 items

  Scenario: Ambiguous step
    Then I am ambiguous

  @skip_on_ci
  Scenario: Printer test skipped by Before hook on CI
    When I add 1 items
    Then the cart has 1 items

  Scenario: Screenshot attachments of every kind
    Then I take a screenshot as "png"
    And I take a screenshot as "jpg"
    And I take a screenshot as "svg"
    And I take a screenshot as "pdf"
    And I take a screenshot as "json"
    And I take a screenshot as "html"
    And I attach a text log

  Scenario: Failure with screenshots attached
    Then I take a screenshot as "png"
    And I take a screenshot as "html"
    And the step "render" fails

  Scenario Outline: Search term <term> <result>
    Then the step "<term>" <result>

    Examples:
      | term | result |
      | term1 | passes |
      | term2 | passes |
      | term3 | passes |
      | term4 | fails |
      | term5 | passes |
      | term6 | passes |
      | term7 | passes |
      | term8 | passes |
      | term9 | passes |
      | term10 | passes |
      | term11 | fails |
      | term12 | passes |
      | term13 | passes |
      | term14 | passes |
      | term15 | passes |
