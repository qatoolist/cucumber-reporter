@feature_tag
Feature: Gherkin shapes

  @outline_tag
  Scenario Outline: Login as <role> with <count> items
    Given a user with role "<role>"
    Then they have <count> items

    Examples: admins
      | role  | count |
      | admin | 3     |

    @ex_tag
    Examples: guests
      | role  | count |
      | guest | 0     |
      | guest | 9     |

  Scenario: Doc string and table
    Given a doc string:
      """json
      {"hello": "world"}
      """
    And a table:
      | a | b |
      | 1 | 2 |

  Rule: Business rule one

    Background:
      Given rule setup

    Example: Under rule passes
      Given a user with role "x"

    Example: Under rule fails
      Then they have 1 items
