Feature: The Gimble application works through a browser

  These scenarios run against its development server and standalone production
  binary.

  Scenario: Project runs can be filtered and opened
    Given I open the project runs
    Then recorded runs show identity, status and time
    When I filter runs by the first recorded status
    Then only runs with that status remain
    When I open the first filtered run
    Then the run workspace shows its identity and observation
    When I select recorded work in the workspace
    Then the detail pane describes that selected work

  Scenario: Navigation and a direct deep link both reach About
    Given I open the project runs
    When I follow the About link
    Then About is visible without a document reload
    When I load the About route directly
    Then About is visible in a new document
