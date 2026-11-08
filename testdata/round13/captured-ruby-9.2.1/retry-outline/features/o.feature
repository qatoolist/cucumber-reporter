Feature: Retry outline

  Scenario Outline: Row <row>
    Given row <row> behaves <how>

    Examples: first
      | row | how    |
      | 1   | pass   |
      | 2   | flaky  |
      | 3   | broken |

    Examples: second
      | row | how    |
      | 4   | flaky  |
      | 5   | pending_after |
