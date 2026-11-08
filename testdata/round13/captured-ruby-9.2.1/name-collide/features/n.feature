Feature: Name collisions

  Scenario Outline: Same outline
    Given value <v>

    Examples:
      | v  |
      | ok |
      | ok |

  Scenario Outline: Same outline
    Given value <v>

    Examples:
      | v   |
      | ok  |
      | bad |

  Scenario: Same outline
    Given value ok
