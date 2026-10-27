import utest.Assert;
import utest.Runner;
import utest.Test;
import utest.ui.Report;

class TestMain {
	static function main() {
		var runner = new Runner();
		runner.addCase(new AddTest());
		Report.create(runner);
		runner.run();
	}
}

class AddTest extends Test {
	function testAdd() {
		Assert.equals(5, add.Add.add(2, 3));
	}
}
