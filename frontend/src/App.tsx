import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type MouseEvent as ReactMouseEvent,
} from "react";
import { Button, Flex, Heading, Text } from "@radix-ui/themes";

const size = 512;
const total = size * size;

import styles from "./App.module.scss";
import { PlayIcon, SymbolIcon } from "@radix-ui/react-icons";

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
const sendSplatter = async (id: string, x: number, y: number) => {
  await fetch(`${import.meta.env.VITE_BACKEND}/signal/${id}/splatter`, {
    method: "POST",
    body: JSON.stringify({ x, y, size: 10 }),
  });
};

export default function App() {
  const [loading, setLoading] = useState(false);
  const [id, setId] = useState<string | null>(null);
  const [population, setPopulation] = useState(0);
  const [time, setTime] = useState(0);

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

  const start = async () => {
    setLoading(true);

    const id = await startWorkflow();
    initialize(id);
  };

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

      sendSplatter(id, x, y);
    },
    [id]
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
        top="4"
        left="4"
        width="240px"
      >
        <Flex align="center" justify="between">
          <Text size="2">Population</Text>
          <Heading>{population.toLocaleString()}</Heading>
        </Flex>
        <Flex align="center" justify="between">
          <Text size="2">Time</Text>
          <Heading>{time.toLocaleString()}</Heading>
        </Flex>
      </Flex>
      <Flex
        gap="2"
        p="2"
        className={styles.control}
        position="fixed"
        bottom="4"
      >
        <Button onClick={() => start()} loading={loading} disabled={loading}>
          {id ? "Restart" : "Start"}
          {id ? <SymbolIcon /> : <PlayIcon />}
        </Button>
      </Flex>
    </Flex>
  );
}
