Feature: The Gimble guide works through a browser

  These scenarios run against its development server and standalone production
  binary.

  Scenario: The guide explains the current product
    Given I open the Gimble guide
    Then the guide says Gimble runs agent workflows in Go
    And the guide says what Gimble does not do

  Scenario: Navigation and a direct deep link both reach About
    Given I open the Gimble guide
    When I follow the About link
    Then About is visible without a document reload
    When I load the About route directly
    Then About is visible in a new document

  Scenario: A selected run opens in the live workspace
    Given I open the selected Gimble run
    Then the run workspace shows its identity and observation
    When I select recorded work in the workspace
    Then the detail pane describes that selected work
