import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type MouseEvent as ReactMouseEvent,
} from "react";
import {
  Button,
  Flex,
  Heading,
  SegmentedControl,
  Separator,
  Text,
} from "@radix-ui/themes";

const size = 512;
const total = size * size;

import styles from "./App.module.scss";
import { InfoCircledIcon, PauseIcon, PlayIcon } from "@radix-ui/react-icons";

/**
 * Start a new workflow (game) on the backend
 */
const startWorkflow = async () => {
  const response = await fetch(`${import.meta.env.VITE_BACKEND}/start`, {
    method: "POST",
  });

  const data = await response.text();

  return data;
};

/**
 * Send a splatter event to the backend
 */
const sendSplatter = async (
  id: string,
  x: number,
  y: number,
  size: number,
  shape: "circle" | "square"
) => {
  await fetch(`${import.meta.env.VITE_BACKEND}/signal/${id}/splatter`, {
    method: "POST",
    body: JSON.stringify({ x, y, size, shape }),
  });
};

export default function App() {
  const [loading, setLoading] = useState(false);

  const [id, setId] = useState<string | null>(null);
  const [population, setPopulation] = useState(0);
  const [time, setTime] = useState(0);

  const [shape] = useState<"circle" | "square">("circle");
  const [radius, setRadius] = useState<"small" | "medium" | "large">("medium");

  const mouseDown = useRef(false);
  const tick = useRef(false);

  const board = useRef<Uint8Array>(null);
  const canvas = useRef<HTMLCanvasElement | null>(null);
  const eventSource = useRef<EventSource | null>(null);

  /**
   * Paint the current board state to the canvas
   */
  const paint = useCallback(() => {
    if (!board.current) return;

    const element = canvas.current;
    if (!element) return;

    const context = element.getContext("2d");
    if (!context) return;

    const imageData = context.createImageData(size, size);
    const { data } = imageData;

    // Fast pixel buffer fill
    let i = 0;
    for (let j = 0; j < total; j++) {
      data[i++] = board.current[j] ? 142 : 255; // R
      data[i++] = board.current[j] ? 140 : 255; // G
      data[i++] = board.current[j] ? 153 : 255; // B
      data[i++] = board.current[j] ? 255 : 0; // A
    }

    context.putImageData(imageData, 0, 0);
  }, []);

  /**
   * Handle window resize to scale canvas appropriately
   */
  useEffect(() => {
    const element = canvas.current;
    if (!element) return;

    const handleResize = () => {
      const multiplier =
        (Math.min(window.innerWidth, window.innerHeight) / size) *
        devicePixelRatio;

      element.style.transform = `scale(${multiplier}) translate(-50%, -50%)`;
    };

    handleResize();

    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  /**
   * Initialize EventSource connection to backend
   */
  const initialize = useCallback(
    (id: string) => {
      eventSource.current?.close();

      board.current = new Uint8Array(total).map(() => 0);

      setTime(0);
      setPopulation(0);

      paint();

      eventSource.current = new EventSource(
        import.meta.env.VITE_BACKEND + "/state/" + id
      );

      eventSource.current.addEventListener("error", () => {
        eventSource.current?.close();

        setId(null);
        setLoading(false);
      });

      // Listen for state updates
      eventSource.current.addEventListener("message", (event) => {
        if (!board.current) return;

        tick.current = false;

        const data = JSON.parse(event.data) as {
          step: number;
          flipped: [number, number][] | null;
        };

        if (!data.flipped) return;

        setTime(data.step);

        for (const [x, y] of data.flipped) {
          const index = y * size + x;
          board.current[index] = board.current[index] ? 0 : 1;
        }

        setPopulation(board.current.reduce((a, b) => a + b, 0));

        paint();
      });

      eventSource.current.addEventListener("open", () => {
        setId(id);
        setLoading(false);
      });
    },
    [paint]
  );

  /**
   * Clean up on unmount
   */
  useEffect(() => {
    return () => {
      eventSource.current?.close();
    };
  }, [initialize]);

  const start = useCallback(async () => {
    console.log("start/stop clicked");

    // End the current workflow (game)
    if (id) {
      eventSource.current?.close();

      setId(null);

      return;
    }

    setLoading(true);

    const workflowId = await startWorkflow();
    initialize(workflowId);
  }, [initialize, id]);

  const handleMouseMove = useCallback(
    (event: MouseEvent) => {
      if (!mouseDown.current) return;

      if (!canvas.current || !id) return;

      if (tick.current) return;
      tick.current = true;

      const { left, top } = canvas.current.getBoundingClientRect();
      const { clientX, clientY } = event;

      const multiplier =
        (Math.min(window.innerWidth, window.innerHeight) / size) *
        devicePixelRatio;

      const x = Math.round((clientX - left) / multiplier);
      const y = Math.round((clientY - top) / multiplier);

      sendSplatter(
        id,
        x,
        y,
        radius === "small" ? 5 : radius === "medium" ? 10 : 20,
        shape
      );
    },
    [id, radius, shape]
  );

  useEffect(() => {
    const handleMouseUp = () => {
      mouseDown.current = false;
    };

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);

    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };
  }, [handleMouseMove]);

  const handleMouseDown = useCallback(
    (event: ReactMouseEvent) => {
      mouseDown.current = true;
      handleMouseMove(event.nativeEvent);
    },
    [handleMouseMove]
  );

  return (
    <Flex align="center" direction="column" justify="center">
      <canvas
        className={styles.canvas}
        ref={canvas}
        width={size}
        height={size}
        onMouseDown={handleMouseDown}
      />
      <Flex
        gap="2"
        py="2"
        px="3"
        direction="column"
        className={styles.panel}
        position="fixed"
        bottom="4"
        left="4"
        width="240px"
      >
        <Flex align="center" justify="between">
          <Text size="2" color="gray">
            Population
          </Text>
          <Heading size="3">{population.toLocaleString()}</Heading>
        </Flex>
        <Separator size="4" />
        <Flex align="center" justify="between">
          <Text size="2" color="gray">
            Time
          </Text>
          <Heading size="3">{time.toLocaleString()}</Heading>
        </Flex>
      </Flex>
      <Flex position="fixed" top="4" left="4" gap="3" direction="column">
        <Flex gap="2" p="2" className={styles.panel} align="center">
          <Button onClick={() => start()} loading={loading} disabled={loading}>
            {id ? "Stop" : "Start"}
            {id ? <PauseIcon /> : <PlayIcon />}
          </Button>
          {/* <Separator orientation="vertical" />
          <Flex align="center" gap="3">
            <Tooltip content="Shape">
              <BorderSplitIcon />
            </Tooltip>
            <SegmentedControl.Root
              disabled={!id}
              value={shape}
              onValueChange={(value) => setShape(value as "circle" | "square")}
            >
              <SegmentedControl.Item value="circle">Circle</SegmentedControl.Item>
              <SegmentedControl.Item value="square">Square</SegmentedControl.Item>
            </SegmentedControl.Root>
          </Flex> */}
          <Separator orientation="vertical" />
          <Flex align="center" gap="3" pl="1">
            <Text size="2" color="gray">
              Splatter size
            </Text>
            <SegmentedControl.Root
              disabled={!id}
              value={radius}
              onValueChange={(value) =>
                setRadius(value as "small" | "medium" | "large")
              }
            >
              <SegmentedControl.Item value="small">Small</SegmentedControl.Item>
              <SegmentedControl.Item value="medium">
                Medium
              </SegmentedControl.Item>
              <SegmentedControl.Item value="large">Large</SegmentedControl.Item>
            </SegmentedControl.Root>
          </Flex>
        </Flex>
        <Text size="2" color="gray">
          <Flex align="center" gap="3" as="span">
            <InfoCircledIcon />
            Click and drag on the canvas to splatter cells
          </Flex>
        </Text>
      </Flex>
    </Flex>
  );
}
