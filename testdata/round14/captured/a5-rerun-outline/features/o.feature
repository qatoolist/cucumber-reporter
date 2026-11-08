Feature: Outline rerun
  Scenario Outline: Row <k>
    Given outcome "<k>"

    Examples:
      | k |
      | R1 |
      | R2 |
      | R3 |

  Scenario Outline: Same name rows
    Given outcome "<k>"

    Examples:
      | k |
      | S1 |
      | S2 |
